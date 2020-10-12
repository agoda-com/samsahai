package webhook

import (
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
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

	retry := false
	for {
		if err = h.samsahai.DestroyActiveEnvironment(team.Name, activeNamespace); err != nil {
			if s2herrors.IsNamespaceStillExists(err) || !retry {
				retry = true
				continue
			}
		}
		break
	}

	if err != nil {
		h.JSON(w, http.StatusInternalServerError, MessageResp{
			Message: fmt.Sprintf("delete active environment failed"),
		})
	} else {
		h.JSON(w, http.StatusOK, MessageResp{
			Message: "success",
		})
	}

}
