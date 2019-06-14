package publicregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/agoda-com/samsahai/internal/util/http"
)

const (
	quayioAPIURL = "https://quay.io/api/v1/repository"

	quayioDomain = "quay.io"
)

type quayioJSON struct {
	HasAdditional bool            `json:"has_additional"`
	Page          int             `json:"page"`
	Tags          []quayioJSONTag `json:"tags,omitempty"`
}
type quayioJSONTag struct {
	Tag string `json:"name"`
	//LastUpdated time.Time `json:"last_modified,omitempty"`
}

// QuayIOFindTag returns matched tag from quay.io
func (c *checker) QuayIOFindTag(ctx context.Context, repo string, matcher *regexp.Regexp) (<-chan string, <-chan error) {
	tagCh := make(chan string)
	errCh := make(chan error)

	go func() {
		log := c.log.WithName(quayioDomain)
		page := 1

		for {
			reqURL := fmt.Sprintf("%s/%s/tag/?onlyActiveTags=true&page=%d", quayioAPIURL, repo, page)

			data, err := http.Get(reqURL, http.WithTimeout(MaxOneRequestTimeout), http.WithContext(ctx))
			if err != nil {
				log.Error(err, "GET request failed", "url", reqURL)
				errCh <- err
				return
			}

			var respJSON quayioJSON
			if err := json.Unmarshal(data, &respJSON); err != nil {
				log.Error(err, "cannot unmarshal json response from")
				errCh <- err
				return
			}

			for _, tag := range respJSON.Tags {
				if matcher.MatchString(tag.Tag) {
					tagCh <- tag.Tag
					return
				}
			}

			if !respJSON.HasAdditional {
				errCh <- fmt.Errorf("no pattern: '%s' match in '%s'", matcher, repo)
				return
			}
			page++
		}
	}()

	return tagCh, errCh
}
