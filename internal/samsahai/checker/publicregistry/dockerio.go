package publicregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
)

const (
	dockerioAPIURL = "https://hub.docker.com/v2/repositories"

	dockerioDomain = "docker.io"
)

type dockerioJSON struct {
	Count int               `json:"count"`
	Next  string            `json:"next"`
	Tags  []dockerioJSONTag `json:"results,omitempty"`
}
type dockerioJSONTag struct {
	Tag         string    `json:"name"`
	LastUpdated time.Time `json:"last_updated"`
}

// DockerHubFindTag returns matched tag from docker.io (hub.docker.com)
func (c *checker) DockerHubFindTag(ctx context.Context, repository string, matcher *regexp.Regexp) (<-chan string, <-chan error) {
	tagCh := make(chan string)
	errCh := make(chan error)

	go func() {
		logger := s2hlog.Log.WithName(dockerioDomain)
		reqURL := fmt.Sprintf("%s/%s/tags/?page=1", dockerioAPIURL, repository)
		var data []byte
		var err error

		for {
			data, err = http.Get(reqURL, http.WithTimeout(MaxOneRequestTimeout), http.WithContext(ctx))
			if err != nil {
				logger.Error(err, "GET request failed", "url", reqURL)
				errCh <- err
				return
			}

			var respJSON dockerioJSON
			if err = json.Unmarshal(data, &respJSON); err != nil {
				logger.Error(err, "cannot unmarshal json response")
				errCh <- err
				return
			}

			for _, tag := range respJSON.Tags {
				if matcher.MatchString(tag.Tag) {
					tagCh <- tag.Tag
					return
				}
			}

			if respJSON.Next == "" {
				logger.Error(fmt.Errorf("no pattern: '%s' match in '%s'", matcher, repository), "")
				errCh <- s2herrors.ErrImageVersionNotFound
				return
			}
			reqURL = respJSON.Next
		}
	}()

	return tagCh, errCh
}
