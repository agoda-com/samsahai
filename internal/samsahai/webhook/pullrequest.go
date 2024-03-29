package webhook

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/julienschmidt/httprouter"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/agoda-com/samsahai/api/v1"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

const validK8sNamePattern = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`

type Components struct {
	Name string `json:"name"`
	Tag  string `json:"tag,omitempty"`
}

type pullRequestWebhookEventJSON struct {
	BundleName       string                          `json:"bundleName"`
	PRNumber         intstr.IntOrString              `json:"prNumber"`
	CommitSHA        string                          `json:"commitSHA,omitempty"`
	Components       []Components                    `json:"components,omitempty"`
	TearDownDuration *v1.PullRequestTearDownDuration `json:"tearDownDuration,omitempty"`
	TestRunner       *v1.ConfigTestRunnerOverrider   `json:"testRunner,omitempty"`
}

type teamPRQueueJSON struct {
	// +optional
	NoOfQueue int `json:"noOfQueue"`

	// +Optional
	Current *v1.PullRequestQueue `json:"current"`

	// +Optional
	Queues []v1.PullRequestQueue `json:"queues"`

	Histories []string `json:"historyNames"`
}

// pullRequestWebhook godoc
// @Summary Webhook For Pull Request Deployment
// @Description Endpoint for manually triggering pull request deployment.
// @Description If testRunner.gitlab.inferBranch is true and testRunner.gitlab.branch is not set,
// @Description it will always try to infer branch regardless of the branch in config.
// @Tags POST
// @Param team path string true "Team name"
// @Accept  json
// @Produce  json
// @Param pullRequestWebhookEventJSON body webhook.pullRequestWebhookEventJSON true "Pull Request"
// @Success 204 {string} string
// @Failure 400 {object} errResp "Invalid JSON"
// @Failure 500 {object} errResp "Internal Server Errors"
// @Router /teams/{team}/pullrequest/trigger [post]
func (h *handler) pullRequestWebhook(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	teamName := params.ByName("team")

	data, err := h.readRequestBody(w, r)
	if err != nil {
		return
	}

	var jsonData pullRequestWebhookEventJSON
	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		h.error(w, http.StatusBadRequest, s2herrors.ErrInvalidJSONData)
		return
	}

	if jsonData.BundleName == "" || jsonData.PRNumber.String() == "" {
		h.error(w, http.StatusBadRequest, fmt.Errorf("must define bundleName and prNumber"))
		return
	}

	matched, err := regexp.Match(validK8sNamePattern, []byte(jsonData.PRNumber.String()))
	if err != nil || !matched {
		h.error(w, http.StatusBadRequest, fmt.Errorf("invalid prNumber, must match regex %s", validK8sNamePattern))
		return
	}

	// if infer branch is true, override branch to prefer branch inference
	if jsonData.TestRunner != nil &&
		jsonData.TestRunner.Gitlab != nil &&
		jsonData.TestRunner.Gitlab.InferBranch != nil &&
		*jsonData.TestRunner.Gitlab.InferBranch &&
		jsonData.TestRunner.Gitlab.Branch == nil {

		branch := ""
		jsonData.TestRunner.Gitlab.Branch = &branch
	}

	mapCompTag := make(map[string]string)
	for _, comp := range jsonData.Components {
		mapCompTag[comp.Name] = comp.Tag
	}

	err = h.samsahai.TriggerPullRequestDeployment(teamName, jsonData.BundleName, jsonData.PRNumber.String(),
		jsonData.CommitSHA, mapCompTag, jsonData.TearDownDuration, jsonData.TestRunner)
	if err != nil {
		if s2herrors.IsErrPullRequestBundleNotFound(err) {
			h.error(w, http.StatusBadRequest,
				fmt.Errorf("there is no '%s' bundle name exists in configuration", jsonData.BundleName))
			return
		}

		h.error(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getTeamPullRequestQueue godoc
// @Summary Get Team's Pull Request Queues
// @Description Returns queue information of pull request deployment flow.
// @Tags GET
// @Param team path string true "Team name"
// @Success 200 {object} teamPRQueueJSON
// @Failure 404 {object} errResp "Team not found"
// @Failure 500 {object} errResp
// @Router /teams/{team}/pullrequest/queue [get]
func (h *handler) getTeamPullRequestQueue(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	prQueues, err := h.samsahai.GetPullRequestQueues(team.Status.Namespace.Staging)
	if err != nil {
		h.errorf(w, http.StatusInternalServerError, "cannot list pull request queues: %+v", err)
		return
	}
	histories, err := h.samsahai.GetPullRequestQueueHistories(team.Status.Namespace.Staging)
	if err != nil {
		h.errorf(w, http.StatusInternalServerError, "cannot list pull request queues: %+v", err)
		return
	}
	data := teamPRQueueJSON{
		NoOfQueue: len(prQueues.Items),
		Queues:    prQueues.Items,
	}

	if len(histories.Items) > 0 {
		for _, history := range histories.Items {
			data.Histories = append(data.Histories, history.Name)
		}
	}
	for i, prQueue := range prQueues.Items {
		if prQueue.Status.State != v1.PullRequestQueueWaiting {
			data.Current = &prQueues.Items[i]
		}
	}

	h.JSON(w, http.StatusOK, &data)
}

// getTeamPullRequestQueueHistory godoc
// @Summary Get Team Pull Request Queue History
// @Description Return pull request queue history of team by id
// @Tags GET
// @Param team path string true "Team name"
// @Param queue path string true "pull request queue history name"
// @Success 200 {object} v1.PullRequestQueueHistory
// @Failure 404 {object} errResp "Team not found"
// @Failure 404 {object} errResp "pull request queue history not found"
// @Failure 500 {object} errResp
// @Router /teams/{team}/pullrequest/queue/histories/{queue} [get]
func (h *handler) getTeamPullRequestQueueHistory(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	prQueueHistoryName := params.ByName("queue")
	if prQueueHistoryName == "" || team.Status.Namespace.Staging == "" {
		h.error(w, http.StatusNotFound, fmt.Errorf("pull request queue history %s in team %s not found",
			prQueueHistoryName, team.Name))
		return
	}

	qh, err := h.samsahai.GetPullRequestQueueHistory(prQueueHistoryName, team.Status.Namespace.Staging)
	if err != nil {
		if errors.IsNotFound(err) {
			h.error(w, http.StatusNotFound, fmt.Errorf("pull request queue history %s in team %s not found",
				prQueueHistoryName, team.Name))
			return
		}
		h.error(w, http.StatusInternalServerError, fmt.Errorf("cannot get team: %+v", err))
		return
	}

	h.JSON(w, http.StatusOK, qh)
}

// getTeamPullRequestQueueHistoryLog godoc
// @Summary Get Team Pull Request Queue History Log
// @Description Returns zip log file of the pull request queue history
// @Tags GET
// @Param team path string true "Team name"
// @Param queue path string true "pull request queue history name"
// @Success 200 {object} v1.PullRequestQueueHistory
// @Failure 404 {object} errResp "Team not found"
// @Failure 404 {object} errResp "pull request queue history not found"
// @Failure 500 {object} errResp
// @Router /teams/{team}/pullrequest/queue/histories/{queue}/log [get]
func (h *handler) getTeamPullRequestQueueHistoryLog(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	prQueueHistoryName := params.ByName("queue")
	if prQueueHistoryName == "" || team.Status.Namespace.Staging == "" {
		h.error(w, http.StatusNotFound, fmt.Errorf("pull request queue history %s in team %s not found",
			prQueueHistoryName, team.Name))
		return
	}

	prQueueHist, err := h.samsahai.GetPullRequestQueueHistory(prQueueHistoryName, team.Status.Namespace.Staging)
	if err != nil {
		if errors.IsNotFound(err) {
			h.error(w, http.StatusNotFound, fmt.Errorf("pull request queue history %s in team %s not found",
				prQueueHistoryName, team.Name))
			return
		}
		h.error(w, http.StatusInternalServerError, fmt.Errorf("cannot get team: %+v", err))
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-log.zip", prQueueHist.Name))
	if prQueueHist.Spec.PullRequestQueue == nil || prQueueHist.Spec.PullRequestQueue.Status.DeploymentQueue == nil {
		h.error(w, http.StatusInternalServerError, fmt.Errorf("logs not found"))
		return
	}

	deploymentQueue := prQueueHist.Spec.PullRequestQueue.Status.DeploymentQueue
	data, err := base64.URLEncoding.DecodeString(deploymentQueue.Status.KubeZipLog)
	if err != nil {
		h.error(w, http.StatusInternalServerError, fmt.Errorf("cannot decode zip log from base64: %+v", err))
		return
	}
	_, _ = w.Write(data)
}
