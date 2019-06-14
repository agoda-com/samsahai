package publicregistry

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/agoda-com/samsahai/internal"

	"github.com/docker/distribution/reference"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	CheckerName = "public-registry"

	MaxRequestsTimeout   = 60 * time.Second
	MaxOneRequestTimeout = 10 * time.Second
)

type checker struct {
	log logr.Logger
}

func New() internal.DesiredComponentChecker {
	return &checker{
		log: log.Log.WithName(CheckerName),
	}
}

func (c *checker) GetName() string {
	return CheckerName
}

func (c *checker) GetVersion(repository, name, pattern string) (string, error) {
	named, err := reference.ParseNormalizedNamed(repository)
	if err != nil {
		return "", err
	}

	matcher, err := regexp.Compile(pattern)
	if err != nil {
		c.log.Error(err, "invalid pattern", "pattern", pattern)
		return "", err
	}

	domain := reference.Domain(named)
	repo := reference.Path(named)
	var tagCh <-chan string
	var errCh <-chan error

	ctx, cancelFunc := context.WithTimeout(context.Background(), MaxRequestsTimeout)
	defer cancelFunc()

	switch domain {
	case dockerioDomain:
		tagCh, errCh = c.DockerHubFindTag(ctx, repo, matcher)
	case quayioDomain:
		tagCh, errCh = c.QuayIOFindTag(ctx, repo, matcher)
	default:
		return "", fmt.Errorf("repository not supported: %s", domain)
	}

	select {
	case <-ctx.Done():
		err := fmt.Errorf("request timeout")
		c.log.Error(err, fmt.Sprintf("checking took more than %v", MaxRequestsTimeout))
		return "", err
	case err := <-errCh:
		return "", err
	case tag := <-tagCh:
		return tag, nil
	}
}
