package webhook

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

type githubPushEventJSON struct {
	Ref        string `json:"ref"`
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// githubWebhook godoc
// @Summary Github Webhook
// @Description Endpoint for receiving webhook notifications from Github
// @Tags POST
// @Produce  json
// @Param GithubPushEvent body webhook.githubPushEventJSON true "Github Push Event"
// @Success 204 {string} string
// @Failure 400 {object} errResp "Invalid JSON"
// @Router /webhook/github [post]
func (h *handler) githubWebhook(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	data, err := h.readRequestBody(w, r)
	if err != nil {
		return
	}

	var jsonData githubPushEventJSON
	err = json.Unmarshal(data, &jsonData)
	if err != nil || jsonData.Ref == "" || jsonData.Repository.Name == "" || jsonData.Repository.FullName == "" {
		h.error(w, http.StatusBadRequest, s2herrors.ErrInvalidJSONData)
		return
	}
	branchName := jsonData.Ref
	if strings.Contains(branchName, "refs/heads/") {
		branchName = strings.Split(jsonData.Ref, "refs/heads/")[1]
	}
	h.samsahai.NotifyGitChanged(internal.GitInfo{
		Name:       jsonData.Repository.Name,
		FullName:   jsonData.Repository.FullName,
		BranchName: branchName,
	})

	w.WriteHeader(http.StatusNoContent)
}
