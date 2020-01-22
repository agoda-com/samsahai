package staging

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"

	s2h "github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	"github.com/agoda-com/samsahai/pkg/staging/rpc"
)

func ErrorComponentNotExist(name string) error {
	return fmt.Errorf("component `%s` is not exist", name)
}

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

func (c *controller) UpdateConfiguration(ctx context.Context, config *rpc.Configuration) (*rpc.Empty, error) {
	if err := c.authenticateRPC(ctx.Value(s2h.HTTPHeader(s2h.SamsahaiAuthHeader)).(string)); err != nil {
		return nil, err
	}
	cfg := &s2h.Configuration{}
	err := json.Unmarshal(config.Config, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal configuration")
	}
	c.getConfigManager().Load(cfg, config.GitRevision)
	logger.Debug("configuration updated", "gitRevision", config.GitRevision)
	return &rpc.Empty{}, nil
}

func (c *controller) authenticateRPC(authToken string) error {
	isMatch := subtle.ConstantTimeCompare([]byte(authToken), []byte(c.authToken))
	if isMatch != 1 {
		return s2herrors.ErrUnauthorized
	}
	return nil
}
