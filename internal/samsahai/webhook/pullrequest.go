package webhook

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"k8s.io/apimachinery/pkg/util/intstr"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

type pullRequestWebhookEventJSON struct {
	Component string             `json:"component"`
	PRNumber  intstr.IntOrString `json:"prNumber"`
	Tag       string             `json:"tag,omitempty"`
}

// pullRequestWebhook godoc
// @Summary Webhook For Pull Request Deployment
// @Description Endpoint for manually triggering pull request deployment
// @Tags POST
// @Accept  json
// @Produce  json
// @Param pullRequestWebhookEventJSON body webhook.pullRequestWebhookEventJSON true "Pull Request"
// @Success 204 {string} string
// @Failure 400 {object} errResp "Invalid JSON"
// @Failure 500 {object} errResp "Internal Server Errors"
// @Router /teams/{team}/pullrequest/queue [post]
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

	err = h.samsahai.TriggerPullRequestDeployment(teamName, jsonData.Component, jsonData.Tag,
		jsonData.PRNumber.String())
	if err != nil {
		h.error(w, http.StatusInternalServerError, err)
	}

	w.WriteHeader(http.StatusNoContent)
}
