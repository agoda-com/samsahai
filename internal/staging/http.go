package staging

import (
	"context"
	"net/http"

	"github.com/julienschmidt/httprouter"

	s2h "github.com/agoda-com/samsahai/internal"
)

func (c *controller) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := context.WithValue(req.Context(), s2h.HTTPHeader(s2h.SamsahaiAuthHeader), req.Header.Get(s2h.SamsahaiAuthHeader))
	req = req.WithContext(ctx)

	router := httprouter.New()

	router.GET("/healthz", func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		b := []byte(`{"msg":"ok"}`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	})

	mux := http.NewServeMux()
	mux.Handle(c.rpcHandler.PathPrefix(), c.rpcHandler)
	mux.Handle("/", router)
	mux.ServeHTTP(w, req)
}
