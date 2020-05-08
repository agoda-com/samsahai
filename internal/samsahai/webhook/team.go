package webhook

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"k8s.io/apimachinery/pkg/api/errors"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
)

type teamsJSON struct {
	Teams []string `json:"teams"`
}

// getTeams godoc
// @Summary Get Teams
// @Description Returns a list of teams that currently running on Samsahai.
// @Tags GET
// @Success 200 {object} teamsJSON
// @Router /teams [get]
func (h *handler) getTeams(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	teamList, err := h.samsahai.GetTeams()
	if err != nil {
		logger.Error(err, "get teams")
		h.errorf(w, http.StatusInternalServerError, "cannot list teams: %+v", err)
		return
	}

	var teams []string
	for _, team := range teamList.Items {
		teams = append(teams, team.Name)
	}

	h.JSON(w, http.StatusOK, &teamsJSON{
		Teams: teams,
	})
}

type teamJSON struct {
	s2hv1.TeamNamespace `json:"namespace"`
	TeamName            string             `json:"teamName"`
	TeamConnections     teamEnvConnections `json:"connections"`
	TeamStatus          s2hv1.TeamStatus   `json:"status"`
	TeamSpec            s2hv1.TeamSpec     `json:"spec"`
	//TeamQueue             teamQueueJSON          `json:"queue"`
}

type teamQueueJSON struct {
	// +optional
	NoOfQueue int `json:"noOfQueue"`

	// +Optional
	Current *s2hv1.Queue `json:"current"`

	// +Optional
	Queues []s2hv1.Queue `json:"queues"`

	Histories []string `json:"historyNames"`
}

type teamEnvConnections struct {
	// +optional
	Staging map[string][]internal.Connection `json:"staging,omitempty"`

	// +optional
	PreActive map[string][]internal.Connection `json:"preActive,omitempty"`

	// +optional
	Active map[string][]internal.Connection `json:"active,omitempty"`
}

// getTeams godoc
// @Summary Get Team
// @Description Returns team information. (namespaces, connections)
// @Tags GET
// @Param team path string true "Team name"
// @Success 200 {object} teamJSON
// @Failure 404 {object} errResp "Team not found"
// @Failure 500 {object} errResp
// @Router /teams/{team} [get]
func (h *handler) getTeam(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	envConnections := teamEnvConnections{}

	if team.Status.Namespace.Staging != "" {
		connections, err := h.samsahai.GetConnections(team.Status.Namespace.Staging)
		if err != nil {
			h.errorf(w, http.StatusInternalServerError, "cannot get staging connections: %+v", err)
			return
		}
		envConnections.Staging = connections
	}
	if team.Status.Namespace.Active != "" {
		connections, err := h.samsahai.GetConnections(team.Status.Namespace.Active)
		if err != nil {
			h.errorf(w, http.StatusInternalServerError, "cannot get active connections: %+v", err)
			return
		}
		envConnections.Active = connections
	}

	if team.Status.Namespace.PreActive != "" {
		connections, err := h.samsahai.GetConnections(team.Status.Namespace.PreActive)
		if err != nil {
			h.errorf(w, http.StatusInternalServerError, "cannot get pre-active connections: %+v", err)
			return
		}
		envConnections.PreActive = connections
	}

	// Get Team info.
	data := teamJSON{
		TeamName:        team.Name,
		TeamConnections: envConnections,
		TeamNamespace:   team.Status.Namespace,
		TeamStatus:      team.Status,
		TeamSpec:        team.Spec,
	}
	data.TeamSpec.Credential = s2hv1.Credential{}
	h.JSON(w, http.StatusOK, &data)
}

type teamComponentsJSON map[string]*s2hv1.Component

// getTeamComponent godoc
// @Summary Get Team Component
// @Description Returns list of components of team
// @Tags GET
// @Param team path string true "Team name"
// @Success 200 {object} teamComponentsJSON
// @Failure 404 {object} errResp "Team not found"
// @Router /teams/{team}/components [get]
func (h *handler) getTeamComponent(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	var data teamComponentsJSON
	teamName := params.ByName("team")

	configCtrl := h.samsahai.GetConfigController()

	var err error
	data, err = configCtrl.GetComponents(teamName)
	if err != nil {
		h.error(w, http.StatusInternalServerError, fmt.Errorf("cannot get conponents of team: %+v", err))
		return
	}

	h.JSON(w, http.StatusOK, data)
}

// getTeamQueue godoc
// @Summary Get Team's Queues
// @Description Returns queue information of new component upgrading flow.
// @Tags GET
// @Param team path string true "Team name"
// @Success 200 {object} teamQueueJSON
// @Failure 404 {object} errResp "Team not found"
// @Failure 500 {object} errResp
// @Router /teams/{team}/queue [get]
func (h *handler) getTeamQueue(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	queues, err := h.samsahai.GetQueues(team.Status.Namespace.Staging)
	if err != nil {
		h.errorf(w, http.StatusInternalServerError, "cannot list queues: %+v", err)
		return
	}
	histories, err := h.samsahai.GetQueueHistories(team.Status.Namespace.Staging)
	if err != nil {
		h.errorf(w, http.StatusInternalServerError, "cannot list queues: %+v", err)
		return
	}
	data := teamQueueJSON{
		NoOfQueue: len(queues.Items),
		Queues:    queues.Items,
	}

	if len(histories.Items) > 0 {
		for _, history := range histories.Items {
			data.Histories = append(data.Histories, history.Name)
		}
	}
	for i, queue := range queues.Items {
		if queue.Status.State != s2hv1.Waiting {
			data.Current = &queues.Items[i]
		}
	}

	h.JSON(w, http.StatusOK, &data)
}

// getTeamQueueHistoryLog godoc
// @Summary Get Team Queue History Log
// @Description Returns zip log file of the queue history
// @Tags GET
// @Param team path string true "Team name"
// @Param queue path string true "Queue history name"
// @Success 200 {object} v1.QueueHistory
// @Failure 404 {object} errResp "Team not found"
// @Failure 404 {object} errResp "Queue history not found"
// @Failure 500 {object} errResp
// @Router /teams/{team}/queue/histories/{queue}/log [get]
func (h *handler) getTeamQueueHistoryLog(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	queueHistoryName := params.ByName("queue")
	if queueHistoryName == "" || team.Status.Namespace.Staging == "" {
		h.error(w, http.StatusNotFound, fmt.Errorf("queue history %s in team %s not found", queueHistoryName, team.Name))
		return
	}

	qh, err := h.samsahai.GetQueueHistory(queueHistoryName, team.Status.Namespace.Staging)
	if err != nil {
		if errors.IsNotFound(err) {
			h.error(w, http.StatusNotFound, fmt.Errorf("queue history %s in team %s not found", queueHistoryName, team.Name))
			return
		}
		h.error(w, http.StatusInternalServerError, fmt.Errorf("cannot get team: %+v", err))
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-log.zip", qh.Name))
	data, err := base64.URLEncoding.DecodeString(qh.Spec.Queue.Status.KubeZipLog)
	if err != nil {
		h.error(w, http.StatusInternalServerError, fmt.Errorf("cannot decode zip log from base64: %+v", err))
		return
	}
	_, _ = w.Write(data)
}

// getTeamQueueHistory godoc
// @Summary Get Team Queue History
// @Description Return queue history of team by id
// @Tags GET
// @Param team path string true "Team name"
// @Param queue path string true "Queue history name"
// @Success 200 {object} v1.QueueHistory
// @Failure 404 {object} errResp "Team not found"
// @Failure 404 {object} errResp "Queue history not found"
// @Failure 500 {object} errResp
// @Router /teams/{team}/queue/histories/{queue} [get]
func (h *handler) getTeamQueueHistory(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	queueHistoryName := params.ByName("queue")
	if queueHistoryName == "" || team.Status.Namespace.Staging == "" {
		h.error(w, http.StatusNotFound, fmt.Errorf("queue history %s in team %s not found", queueHistoryName, team.Name))
		return
	}

	qh, err := h.samsahai.GetQueueHistory(queueHistoryName, team.Status.Namespace.Staging)
	if err != nil {
		if errors.IsNotFound(err) {
			h.error(w, http.StatusNotFound, fmt.Errorf("queue history %s in team %s not found", queueHistoryName, team.Name))
			return
		}
		h.error(w, http.StatusInternalServerError, fmt.Errorf("cannot get team: %+v", err))
		return
	}

	h.JSON(w, http.StatusOK, qh)
}

// getTeamComponentStableValues godoc
// @Summary get team stable component values
// @Description get team stable component values
// @Tags GET
// @Param team path string true "Team name"
// @Param component path string true "Component name"
// @Param accept header string true "Accept" enums(application/json, application/x-yaml)
// @Success 200 {string} v1.ComponentValues
// @Failure 404 {object} errResp "Team not found"
// @Failure 404 {object} errResp "Component not found"
// @Failure 500 {object} errResp
// @Router /teams/{team}/components/{component}/values [get]
func (h *handler) getTeamComponentStableValues(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	componentName := params.ByName("component")
	parentComps, err := h.samsahai.GetConfigController().GetParentComponents(team.Name)
	if err != nil {
		h.errorf(w, http.StatusBadRequest, "cannot get parent components from config of team: %s", team.Name)
		return
	}

	comp, ok := parentComps[componentName]
	if !ok {
		h.errorf(w, http.StatusBadRequest, "invalid component name: %s", componentName)
		return
	}
	values, err := h.samsahai.GetStableValues(team, comp)
	if err != nil {
		logger.Error(err, fmt.Sprintf("cannot get stable value for %s", componentName))
		h.errorf(w,
			http.StatusInternalServerError,
			"cannot get stable value for '%s': %+v", componentName, err)
		return
	}

	switch r.Header.Get("accept") {
	case "application/x-yaml":
		fallthrough
	case "text/yaml":
		h.YAML(w, http.StatusOK, values)
		return
	default:
		h.JSON(w, http.StatusOK, values)
	}
}

// getTeamConfig godoc
// @Summary get team configuration
// @Description get team configuration
// @Tags GET
// @Param team path string true "Team name"
// @Param accept header string true "Accept" enums(application/json, application/x-yaml)
// @Success 200 {string} v1.ConfigSpec
// @Failure 404 {object} errResp "Team not found"
// @Failure 500 {object} errResp
// @Router /teams/{team}/config [get]
func (h *handler) getTeamConfig(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	configCtrl := h.samsahai.GetConfigController()
	config, err := configCtrl.Get(team.Name)
	if err != nil {
		h.errorf(w, http.StatusBadRequest, "team config not found: %s", team.Name)
		return
	}

	switch r.Header.Get("accept") {
	case "application/x-yaml":
		fallthrough
	case "text/yaml":
		h.YAML(w, http.StatusOK, config.Spec)
		return
	default:
		h.JSON(w, http.StatusOK, config.Spec)
	}
}

func (h *handler) loadTeam(w http.ResponseWriter, params httprouter.Params) (*s2hv1.Team, error) {
	teamName := params.ByName("team")

	team := &s2hv1.Team{}
	err := h.samsahai.GetTeam(teamName, team)
	if err != nil {
		if errors.IsNotFound(err) {
			h.error(w, http.StatusNotFound, fmt.Errorf("team %s not found", teamName))
			return nil, err
		}
		h.error(w, http.StatusInternalServerError, fmt.Errorf("cannot get team: %+v", err))
		return nil, err
	}

	return team, nil
}
