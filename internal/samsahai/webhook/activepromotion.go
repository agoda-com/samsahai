package webhook

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
)

type activePromotion struct {
	// +Optional
	Running []v1.ActivePromotion `json:"running"`

	// +Optional
	InQueues []string `json:"inQueues" example:"team1,team2"`
}

// getActivePromotions godoc
// @Summary get current active promotions
// @Description get current active promotions
// @Tags GET
// @Produce  json
// @Success 200 {object} activePromotion
// @Failure 500 {object} errResp "cannot get activepromotions"
// @Router /activepromotions [get]
func (h *handler) getActivePromotions(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	atpList, err := h.samsahai.GetActivePromotions()
	if err != nil {
		h.error(w, http.StatusInternalServerError, fmt.Errorf("cannot get activepromotions: %+v", err))
		return
	}

	atpList.SortASC()

	runningList := make([]v1.ActivePromotion, 0)
	waitingList := make([]string, 0)
	if len(atpList.Items) > 0 {
		for _, atp := range atpList.Items {
			if atp.Status.State == v1.ActivePromotionWaiting {
				waitingList = append(waitingList, atp.Name)
				continue
			}
			runningList = append(runningList, atp)
		}
	}

	data := &activePromotion{
		Running:  runningList,
		InQueues: waitingList,
	}

	h.JSON(w, http.StatusOK, data)
}

type teamActivePromotion struct {
	// +Optional
	Current *v1.ActivePromotion `json:"current"`

	// +Optional
	Histories []string `json:"historyNames" example:"team1-20191010-080000,team1-20191009-080000"`
}

// getTeamActivePromotion godoc
// @Summary get active promotions by team name
// @Description get active promotions by team name
// @Tags GET
// @Produce  json
// @Param team path string true "Team name"
// @Success 200 {object} teamActivePromotion
// @Failure 404 {string} string "team {team} not found"
// @Failure 500 {string} string "cannot get activepromotion/activepromotion histories of team {team}"
// @Router /teams/{team}/activepromotions [get]
func (h *handler) getTeamActivePromotions(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	team, err := h.loadTeam(w, params)
	if err != nil {
		return
	}

	atp, err := h.samsahai.GetActivePromotion(team.Name)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			h.error(w, http.StatusInternalServerError,
				fmt.Errorf("cannot get activepromotion of team %s: %+v", team.Name, err))
			return
		}
		atp = nil
	}

	atpHistList, err := h.getActivePromotionHistoryListByDESC(team.Name)
	if err != nil {
		h.error(w, http.StatusInternalServerError,
			fmt.Errorf("cannot get activepromotion histories of team %s: %+v", team.Name, err))
		return
	}

	histNames := make([]string, 0)
	for _, hist := range atpHistList.Items {
		histNames = append(histNames, hist.Name)
	}

	data := &teamActivePromotion{
		Current:   atp,
		Histories: histNames,
	}

	h.JSON(w, http.StatusOK, data)
}

type activePromotionHistories []v1.ActivePromotionHistory

// getTeamActivePromotion godoc
// @Summary get active promotion histories by team name
// @Description get active promotion histories by team name
// @Tags GET
// @Produce  json
// @Param team path string true "Team name"
// @Success 200 {object} activePromotionHistories
// @Failure 400 {string} string "team should not be empty"
// @Failure 500 {string} string "cannot get activepromotion histories of team {team}"
// @Router /teams/{team}/activepromotions/histories [get]
func (h *handler) getTeamActivePromotionHistories(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	teamName := params.ByName("team")
	if teamName == "" {
		h.error(w, http.StatusBadRequest, fmt.Errorf("team should not be empty"))
	}

	atpHistList, err := h.getActivePromotionHistoryListByDESC(teamName)
	if err != nil {
		h.error(w, http.StatusInternalServerError,
			fmt.Errorf("cannot get activepromotion histories of team %s: %+v", teamName, err))
		return
	}

	atpHists := activePromotionHistories(atpHistList.Items)
	h.JSON(w, http.StatusOK, atpHists)
}

// getTeamActivePromotion godoc
// @Summary get active promotion history by team and history name
// @Description get active promotion history by team and history name
// @Tags GET
// @Produce  json
// @Param team path string true "Team name"
// @Param history path string true "Active promotion history name"
// @Success 200 {string} string
// @Failure 400 {string} string "team/history should not be empty"
// @Failure 404 {string} string "activepromotion history {history} of team {team} not found"
// @Failure 500 {string} string "cannot get activepromotion history {history} of team {team}"
// @Router /teams/{team}/activepromotions/histories/{history} [get]
func (h *handler) getTeamActivePromotionHistory(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	teamName := params.ByName("team")
	if teamName == "" {
		h.error(w, http.StatusBadRequest, fmt.Errorf("team should not be empty"))
	}

	atpHistName := params.ByName("history")
	if atpHistName == "" {
		h.error(w, http.StatusBadRequest, fmt.Errorf("history %s should not be empty", atpHistName))
		return
	}

	atpHist, err := h.samsahai.GetActivePromotionHistory(atpHistName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			h.error(w, http.StatusNotFound,
				fmt.Errorf("activepromotion history %s of team %s not found", atpHistName, teamName))
			return
		}
		h.error(w, http.StatusInternalServerError,
			fmt.Errorf("cannot get activepromotion history %s of team %s: %+v", atpHistName, teamName, err))
		return
	}

	teamKey := internal.GetTeamLabelKey()
	if atpHist.Labels[teamKey] != teamName {
		h.error(w, http.StatusNotFound,
			fmt.Errorf("activepromotion history %s of team %s not found", atpHistName, teamName))
		return
	}

	h.JSON(w, http.StatusOK, atpHist)
}

// getTeamActivePromotionHistoryLog godoc
// @Summary Get zip log of active promotion history
// @Description Returns zip log file of the active promotion history
// @Tags GET
// @Produce  json
// @Param team path string true "Team name"
// @Param history path string true "Active promotion history name"
// @Success 200 {string} string
// @Failure 400 {string} string "team/history should not be empty"
// @Failure 404 {string} string "activepromotion history {history} of team {team} not found"
// @Failure 500 {string} string "cannot get activepromotion history {history} of team {team}"
// @Router /teams/{team}/activepromotions/histories/{history}/log [get]
func (h *handler) getTeamActivePromotionHistoryLog(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	teamName := params.ByName("team")
	if teamName == "" {
		h.error(w, http.StatusBadRequest, fmt.Errorf("team should not be empty"))
	}

	atpHistName := params.ByName("history")
	if atpHistName == "" {
		h.error(w, http.StatusBadRequest, fmt.Errorf("history %s should not be empty", atpHistName))
		return
	}

	atpHist, err := h.samsahai.GetActivePromotionHistory(atpHistName)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			h.error(w, http.StatusNotFound,
				fmt.Errorf("activepromotion history %s of team %s not found", atpHistName, teamName))
			return
		}
		h.error(w, http.StatusInternalServerError,
			fmt.Errorf("cannot get activepromotion history %s of team %s: %+v", atpHistName, teamName, err))
		return
	}

	teamKey := internal.GetTeamLabelKey()
	if atpHist.Labels[teamKey] != teamName {
		h.error(w, http.StatusNotFound,
			fmt.Errorf("activepromotion history %s of team %s not found", atpHistName, teamName))
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-log.zip", atpHist.Name))
	data, err := base64.URLEncoding.DecodeString(atpHist.Spec.ActivePromotion.Status.PreActiveQueue.KubeZipLog)
	if err != nil {
		h.error(w, http.StatusInternalServerError, fmt.Errorf("cannot decode zip log from base64: %+v", err))
		return
	}
	_, _ = w.Write(data)
}

func (h *handler) getActivePromotionHistoryListByDESC(teamName string) (*v1.ActivePromotionHistoryList, error) {
	labels := internal.GetDefaultLabels(teamName)
	atpHistList, err := h.samsahai.GetActivePromotionHistories(labels)
	if err != nil {
		return nil, err
	}

	atpHistList.SortDESC()

	return atpHistList, nil
}
