package harbor

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/docker/distribution/reference"

	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
)

var logger = s2hlog.Log.WithName(CheckerName)

const (
	CheckerName = "harbor"

	MaxRequestsTimeout   = 60 * time.Second
	MaxOneRequestTimeout = 10 * time.Second
)

type checker struct {
	httpOpts []http.Option
}

type harborRes struct {
	Tag string `json:"name"`
}

func New(opts ...http.Option) internal.DesiredComponentChecker {
	return &checker{
		httpOpts: opts,
	}
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

	tagCh, errCh = c.check(ctx, domain, repo, matcher)

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

// DockerHubFindTag returns matched tag from docker.io (hub.docker.com)
func (c *checker) check(ctx context.Context, domain, repository string, matcher *regexp.Regexp) (<-chan string, <-chan error) {
	tagCh := make(chan string)
	errCh := make(chan error)

	go func() {
		reqURL := fmt.Sprintf("https://%s/api/repositories/%s/tags", domain, repository)
		var data []byte
		var err error

		for {
			opts := []http.Option{
				http.WithTimeout(MaxOneRequestTimeout),
				http.WithContext(ctx),
			}
			if len(c.httpOpts) > 0 {
				opts = append(opts, c.httpOpts...)
			}
			_, data, err = http.Get(reqURL, opts...)
			if err != nil {
				logger.Error(err, "GET request failed", "url", reqURL)
				errCh <- err
				return
			}

			var respJSON []harborRes
			if err = json.Unmarshal(data, &respJSON); err != nil {
				logger.Error(err, "cannot unmarshal json response")
				errCh <- err
				return
			}

			var matchedTags []string
			for _, tag := range respJSON {
				if matcher.MatchString(tag.Tag) {
					matchedTags = append(matchedTags, tag.Tag)
				}
			}

			if len(matchedTags) > 0 {
				sort.Sort(internal.SortableVersion(matchedTags))
				tagCh <- matchedTags[len(matchedTags)-1]
				return
			}

			errCh <- s2herrors.ErrImageVersionNotFound
		}
	}()

	return tagCh, errCh
}
