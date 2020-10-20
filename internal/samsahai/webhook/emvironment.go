package webhook

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

type MessageResp struct {
	Message string `json:"message,omitempty"`
}

func (h *handler) deleteTeamActiveEnvironment(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	activeNamespace := team.Status.Namespace.Active
	if activeNamespace == "" {
		h.errorf(w, http.StatusBadRequest, "there is no active namespace to destroy")
		return
	}

	if err := h.samsahai.DeleteTeamActiveEnvironment(team.Name, activeNamespace); err != nil {
		h.JSON(w, http.StatusInternalServerError, MessageResp{
			Message: fmt.Sprintf("delete active environment failed"),
		})
		return
	}

	h.JSON(w, http.StatusOK, MessageResp{
		Message: "success",
	})
}
