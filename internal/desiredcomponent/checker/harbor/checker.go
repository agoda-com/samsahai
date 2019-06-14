package harbor

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/util/http"
)

const (
	CheckerName = "harbor"

	MaxRequestsTimeout   = 60 * time.Second
	MaxOneRequestTimeout = 10 * time.Second
)

type checker struct {
	log logr.Logger
}

type harborRes struct {
	Tag string `json:"name"`
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

	tagCh, errCh = c.check(ctx, domain, repo, matcher)

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

// DockerHubFindTag returns matched tag from docker.io (hub.docker.com)
func (c *checker) check(ctx context.Context, domain, repository string, matcher *regexp.Regexp) (<-chan string, <-chan error) {
	tagCh := make(chan string)
	errCh := make(chan error)

	go func() {
		log := c.log
		reqURL := fmt.Sprintf("https://%s/api/repositories/%s/tags", domain, repository)
		var data []byte
		var err error

		for {
			data, err = http.Get(reqURL, http.WithTimeout(MaxOneRequestTimeout), http.WithContext(ctx))
			if err != nil {
				log.Error(err, "GET request failed", "url", reqURL)
				errCh <- err
				return
			}

			var respJSON []harborRes
			if err = json.Unmarshal(data, &respJSON); err != nil {
				log.Error(err, "cannot unmarshal json response")
				errCh <- err
				return
			}

			matchedTags := []string{}
			for _, tag := range respJSON {
				if matcher.MatchString(tag.Tag) {
					matchedTags = append(matchedTags, tag.Tag)
				}
			}

			if len(matchedTags) > 0 {
				sort.Sort(internal.SortableVersion(matchedTags))
				tagCh <- matchedTags[len(matchedTags)-1]
			}

			errCh <- internal.ErrImageVersionNotFound
		}
	}()

	return tagCh, errCh
}
