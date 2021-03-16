package harbor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
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

	pageSize     = 1
	maximumPages = 100
)

type checker struct {
	httpOpts []http.Option
}

type harborRes struct {
	Tags []harborTag `json:"tags"`
}

type harborTag struct {
	Name string `json:"name"`
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

// check returns matched tag from harbor
func (c *checker) check(ctx context.Context, domain, fullRepository string, matcher *regexp.Regexp) (<-chan string, <-chan error) {
	tagCh := make(chan string)
	errCh := make(chan error)

	go func() {
		project, repository := extractProjectAndRepository(fullRepository)
		if project == "" || repository == "" {
			err := fmt.Errorf("invalid image repository of harbor, expected `<project_name>/<repository_name>`, got %s",
				fullRepository)
			logger.Error(err, "invalid image repository of harbor", "image", fullRepository)
			errCh <- err
			return
		}

		currentPage := 0
		var matchedTags []string
		for {
			currentPage++
			reqURL := fmt.Sprintf("https://%s/api/v2.0/projects/%s/repositories/%s/artifacts?tags=*&page=%d&page_size=%d",
				domain, project, repository, currentPage, pageSize)
			var data []byte
			var err error

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

			for _, artifact := range respJSON {
				for _, tag := range artifact.Tags {
					if matcher.MatchString(tag.Name) {
						matchedTags = append(matchedTags, tag.Name)
					}
				}
			}

			if len(matchedTags) > 0 {
				sort.Sort(internal.SortableVersion(matchedTags))
				tagCh <- matchedTags[len(matchedTags)-1]
				return
			}

			if len(respJSON) == 0 || currentPage >= maximumPages {
				break
			}
		}

		errCh <- s2herrors.ErrImageVersionNotFound
	}()

	return tagCh, errCh
}

func doubleEscapeParam(str string) string {
	return url.QueryEscape(url.QueryEscape(str))
}

func extractProjectAndRepository(fullRepository string) (project, repository string) {
	paths := strings.Split(fullRepository, "/")
	if len(paths) < 2 {
		logger.Error(errors.New("invalid harbor image repository path"), "there is no repository defined",
			"repository", fullRepository)
		return
	}

	project = paths[0]
	repository = strings.Replace(fullRepository, fmt.Sprintf("%s/", project), "", -1)

	// harbor requires double escape
	return project, doubleEscapeParam(repository)
}
