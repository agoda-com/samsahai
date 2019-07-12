package webhook

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/ghodss/yaml"
	"github.com/julienschmidt/httprouter"

	s2h "github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.S2HLog.WithName("webhook")

type handler struct {
	samsahai s2h.SamsahaiController
}

func New(samsahaiCtrl s2h.SamsahaiController) *httprouter.Router {
	h := handler{
		samsahai: samsahaiCtrl,
	}
	r := httprouter.New()
	h.bind(r)
	return r
}

func (h *handler) bind(r *httprouter.Router) {
	r.GET(s2h.URIVersion, h.getVersion)
	r.GET(s2h.URIHealthz, h.getHealthz)

	r.POST("/webhook/github", h.githubWebhook)
	r.POST("/webhook/component", h.newComponentWebhook)

	// route from plugins
	plugins := h.samsahai.GetPlugins()
	for k := range plugins {
		p := plugins[k]
		r.POST(fmt.Sprintf("/webhook/%s", p.GetName()), pluginWebhookFunc(h, p))
	}

	r.GET("/teams", h.getTeams)
	r.GET("/teams/:team", h.getTeam)
	r.GET("/teams/:team/config", h.getTeamConfig)
	r.GET("/teams/:team/components", h.getTeamComponent)
	r.GET("/teams/:team/queue", h.getTeamQueue)
	r.GET("/teams/:team/queue/histories/:queue", h.getTeamQueueHistory)
	r.GET("/teams/:team/queue/histories/:queue/log", h.getTeamQueueHistoryLog)

	r.GET("/teams/:team/components/:component/values", h.getTeamComponentStableValues)

	r.GET("/teams/:team/activepromotions", h.getTeamActivePromotions)
	r.GET("/teams/:team/activepromotions/histories", h.getTeamActivePromotionHistories)
	r.GET("/teams/:team/activepromotions/histories/:history", h.getTeamActivePromotionHistory)
	r.GET("/teams/:team/activepromotions/histories/:history/log", h.getTeamActivePromotionHistoryLog)

	r.GET("/activepromotions", h.getActivePromotions)

	//r.GET("/teams/:team/queue/:queue", h.getTeamQueue)
	//r.GET("/teams/:team/queue/:queue/logs", h.getTeamQueue)

	//r.GET("/teams/:team/upgrade-histories", h.getTeamComponent)

	//r.GET("/teams/:team/parent-components", h.getTeamComponent)
	//r.GET("/teams/:team/parent-components/:component", h.getTeamComponent)

	//r.GET("/teams/:team/", h.getTeamComponent)

	//r.GET("/teams/:team/components/parent", h.getTeamComponent)
}

type versionJSON struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
}

// getVersion godoc
// @Summary Service Version
// @Description Get service version information.
// @Tags GET
// @Produce  json
// @Success 200 {object} versionJSON
// @Router /version [get]
func (h *handler) getVersion(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	h.JSON(w, http.StatusOK, &versionJSON{
		Version:   s2h.Version,
		GitCommit: s2h.GitCommit,
	})
}

type healthCheckJSON struct {
	Msg string `json:"msg" example:"ok"`
}

// getHealth godoc
// @Summary Health check
// @Description Endpoint for server health check
// @Tags GET
// @Accept  json
// @Success 200 {object} healthCheckJSON
// @Router /healthz [get]
func (h *handler) getHealthz(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	h.JSON(w, http.StatusOK, &healthCheckJSON{
		Msg: "ok",
	})
}

func (h *handler) write(w http.ResponseWriter, statusCode int, data []byte) {
	var err error
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(statusCode)
	_, err = w.Write(data)
	if err != nil {
		logger.Error(err, "write response to http")
	}
}

func (h *handler) JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	var b []byte
	var err error
	b, err = json.Marshal(data)
	if err != nil {
		logger.Error(err, "cannot marshal json")
		w.Header().Set("Content-Type", "application/json")
		h.write(w, http.StatusInternalServerError, []byte(fmt.Sprintf(
			`{"err":"%v","msg":"%v"}`,
			s2herrors.ErrInternalError,
			s2herrors.ErrCannotMarshalJSON,
		)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	h.write(w, statusCode, b)
}

func (h *handler) YAML(w http.ResponseWriter, statusCode int, data interface{}) {
	var err error
	var b []byte
	b, err = yaml.Marshal(data)
	if err != nil {
		logger.Error(err, "cannot marshal yaml")
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		h.write(w, http.StatusInternalServerError, []byte(fmt.Sprintf(
			`{"err":"%v","msg":"%v"}`,
			s2herrors.ErrInternalError,
			s2herrors.ErrCannotMarshalYAML,
		)))
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	h.write(w, statusCode, b)
}

type errResp struct {
	Error string `json:"error"`
}

func (h *handler) error(w http.ResponseWriter, statusCode int, err error) {
	v := errResp{
		Error: err.Error(),
	}
	h.JSON(w, statusCode, v)
}

func (h *handler) errorf(w http.ResponseWriter, statusCode int, format string, args ...interface{}) {
	v := errResp{
		Error: fmt.Sprintf(format, args...),
	}
	h.JSON(w, statusCode, v)
}

func (h *handler) readRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Error(err, "cannot read from req.body")
		h.error(w, http.StatusInternalServerError, s2herrors.ErrInternalError)
		return nil, err
	}
	return data, nil
}

type newComponentEventJSON struct {
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
	h.samsahai.NotifyComponentChanged(jsonData.Component, jsonData.Repository)

	w.WriteHeader(http.StatusNoContent)
}
