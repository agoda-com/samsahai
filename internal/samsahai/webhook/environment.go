package webhook

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// deleteTeamActiveEnvironment godoc
// @Summary Delete the current active namespace
// @Description Delete the current active namespace.
// @Tags GET
// @Param team path string true "Team name"
// @Success 200 {string} string
// @Failure 400 {object} errResp "There is no active namespace to destroy"
// @Failure 404 {object} errResp "Team not found"
// @Failure 500 {object} errResp
// @Router /teams/{team}/environment/active/delete [delete]
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
		logger.Error(err, "error while delete active environment", "team", team.Name)
		h.errorf(w, http.StatusInternalServerError, "delete active environment failed, %v", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
