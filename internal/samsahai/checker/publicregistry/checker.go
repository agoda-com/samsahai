package publicregistry

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/docker/distribution/reference"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName(CheckerName)

const (
	CheckerName = "public-registry"

	MaxRequestsTimeout   = 60 * time.Second
	MaxOneRequestTimeout = 10 * time.Second
)

type checker struct{}

func New() internal.DesiredComponentChecker {
	return &checker{}
}

func (c *checker) GetName() string {
	return CheckerName
}

func (c *checker) GetVersion(repository, name, pattern string) (string, error) {
	if pattern == "" {
		pattern = ".*"
	}

	named, err := reference.ParseNormalizedNamed(repository)
	if err != nil {
		return "", err
	}

	matcher, err := regexp.Compile(pattern)
	if err != nil {
		logger.Error(err, "invalid pattern", "pattern", pattern)
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
		logger.Error(s2herrors.ErrRequestTimeout, fmt.Sprintf("checking took more than %v", MaxRequestsTimeout))
		return pattern, s2herrors.ErrRequestTimeout
	case err := <-errCh:
		return pattern, err
	case tag := <-tagCh:
		return tag, nil
	}
}

func (c *checker) EnsureVersion(repository, name, version string) error {
	_, err := c.GetVersion(repository, name, version)
	return err
}
