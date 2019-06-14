package http

import (
	"net/http"

	"github.com/julienschmidt/httprouter"

	"github.com/agoda-com/samsahai/internal"
)

type handler struct {
	ctrl internal.DesiredComponentController
}

func New(router *httprouter.Router, desiredComponent internal.DesiredComponentController) {
	h := &handler{
		ctrl: desiredComponent,
	}
	router.POST("/notify", h.notify)
	router.POST("/notify/:component", h.notify)
}

func (h *handler) notify(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	name := params.ByName("component")
	if name == "" {
		h.ctrl.TryCheck()
	} else {
		h.ctrl.TryCheck(name)
	}
}
