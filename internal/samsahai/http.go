package samsahai

import (
	"context"
	"net/http"

	s2h "github.com/agoda-com/samsahai/internal"
)

func (c *controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithValue(r.Context(), s2h.HTTPHeader(s2h.SamsahaiAuthHeader), r.Header.Get(s2h.SamsahaiAuthHeader))
	r = r.WithContext(ctx)
	c.rpcHandler.ServeHTTP(w, r)
}

func (c *controller) PathPrefix() string {
	return c.rpcHandler.PathPrefix()
}
