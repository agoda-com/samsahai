package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
)

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
			logger.Debug(fmt.Sprintf("webhook request"),
				"plugin", plugin.GetName(),
				"reqData", string(data),
				"component", jsonData.Component,
				"output", compName)
			h.samsahai.NotifyComponentChanged(compName, "", "")
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
