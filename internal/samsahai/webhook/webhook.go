package webhook

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

func (h *handler) readRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Error(err, "cannot read from req.body")
		h.error(w, http.StatusInternalServerError, s2herrors.ErrInternalError)
		return nil, err
	}
	return data, nil
}

var pluginWebhookFunc = func(h *handler, plugin internal.Plugin) func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		data, err := h.readRequestBody(w, r)
		if err != nil {
			return
		}

		var jsonData newComponentEventJSON
		err = json.Unmarshal(data, &jsonData)
		if err != nil {
			h.error(w, http.StatusBadRequest, s2herrors.ErrInvalidJSONData)
			return
		}

		compName := plugin.GetComponentName(jsonData.Component)

		if compName != "" {
			logger.Debug("webhook request",
				"plugin", plugin.GetName(),
				"reqData", string(data),
				"component", jsonData.Component,
				"output", compName)
			h.samsahai.NotifyComponentChanged(compName, "", "")
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

type newComponentEventJSON struct {
	TeamName   string `json:"teamName,omitempty"`
	Component  string `json:"component"`
	Repository string `json:"repository,omitempty"`
}

// newComponentWebhook godoc
// @Summary Webhook New Component
// @Description Endpoint for manually triggering new component update
// @Tags POST
// @Accept  json
// @Produce  json
// @Param newComponentEventJSON body webhook.newComponentEventJSON true "New Component"
// @Success 204 {string} string
// @Failure 400 {object} errResp "Invalid JSON"
// @Router /webhook/component [post]
func (h *handler) newComponentWebhook(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	data, err := h.readRequestBody(w, r)
	if err != nil {
		return
	}

	var jsonData newComponentEventJSON
	err = json.Unmarshal(data, &jsonData)
	if err != nil {
		h.error(w, http.StatusBadRequest, s2herrors.ErrInvalidJSONData)
		return
	}

	h.samsahai.NotifyComponentChanged(jsonData.Component, jsonData.Repository, jsonData.TeamName)

	w.WriteHeader(http.StatusNoContent)
}
