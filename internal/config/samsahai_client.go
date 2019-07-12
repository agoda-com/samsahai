package config

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"github.com/twitchtv/twirp"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

func NewWithSamsahaiClient(client rpc.RPC, teamName string, authToken string) (internal.ConfigManager, error) {
	var err error
	headers := make(http.Header)
	headers.Set(internal.SamsahaiAuthHeader, authToken)
	ctx := context.TODO()
	ctx, err = twirp.WithHTTPRequestHeaders(ctx, headers)
	if err != nil {
		return nil, errors.Wrap(err, "cannot set request header")
	}
	c, err := client.GetConfiguration(ctx, &rpc.Team{Name: teamName})
	if err != nil {
		return nil, errors.Wrap(err, "cannot load configuration from server")
	}

	var cfg internal.Configuration
	err = json.Unmarshal(c.Config, &cfg)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load configuration from server")
	}

	m := manager{
		config:          &cfg,
		gitHeadRevision: c.GitRevision,
	}

	return &m, nil
}
