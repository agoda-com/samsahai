package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
)

var logger = s2hlog.S2HLog.WithName("Github-util")

const requestTimeout = 5 * time.Second

const commitStatusAPI = "%s/api/v3/repos/%s/statuses/%s" // base url, repository, commit SHA

// CommitStatus represents a commit status
type CommitStatus string

const (
	// CommitStatusSuccess represents a success of commit status
	CommitStatusSuccess CommitStatus = "success"
	// CommitStatusSuccess represents a failure of commit status
	CommitStatusFailure CommitStatus = "failure"
)

// Github is the interface of Github using Github REST API
type Github interface {
	// PublishCommitStatus publishes a commit status for a given SHA
	PublishCommitStatus(repository, commitSHA, labelName, targetURL, description string, status CommitStatus) error
}

var _ Github = &Client{}

// Client manages client side of Github REST API
type Client struct {
	baseURL string // e.g., https://github.com
	token   string
}

// NewClient creates a new client of Github
func NewClient(baseURL, token string) *Client {
	client := &Client{
		baseURL: baseURL,
		token:   token,
	}

	return client
}

type bodyReq struct {
	State       string `json:"state"`
	TargetURL   string `json:"target_url"`
	Description string `json:"description"`
	Context     string `json:"context"`
}

// PublishCommitStatus publishes a commit status for a given SHA
func (c *Client) PublishCommitStatus(repository, commitSHA, labelName, targetURL, description string,
	status CommitStatus) error {

	logger.Debug("committing a status check",
		"repository", repository, "commitSHA", commitSHA, "status", status)

	commitStatusAPI := fmt.Sprintf(commitStatusAPI, c.baseURL, repository, commitSHA)

	resCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	ctx, cancelFunc := context.WithTimeout(context.Background(), requestTimeout)
	defer cancelFunc()
	go func() {
		gitToken := fmt.Sprintf("token %s", c.token)

		opts := []http.Option{
			http.WithTimeout(requestTimeout),
			http.WithContext(ctx),
			http.WithHeader("Authorization", gitToken),
		}

		reqJSON := bodyReq{
			State:       string(status),
			TargetURL:   targetURL,
			Description: description,
			Context:     labelName,
		}

		reqBody, err := json.Marshal(reqJSON)
		if err != nil {
			logger.Error(err, "cannot marshal request data", "data", reqBody)
			errCh <- err
			return
		}

		_, res, err := postRequest(commitStatusAPI, reqBody, opts...)
		if err != nil {
			errCh <- err
			return
		}

		resCh <- res
	}()

	select {
	case <-ctx.Done():
		logger.Error(s2herrors.ErrRequestTimeout,
			fmt.Sprintf("publishing commit status to github repository: %s, commitSHA: %s took longer than %v",
				repository, commitSHA, requestTimeout))
		return s2herrors.ErrRequestTimeout
	case err := <-errCh:
		logger.Error(err, "cannot publish commit status",
			"repository", repository, "commitSHA", commitSHA)
		return err
	case <-resCh:
		logger.Info("commit status successfully published to github",
			"repository", repository, "commitSHA", commitSHA)
		return nil
	}
}

func postRequest(reqURL string, body []byte, opts ...http.Option) (int, []byte, error) {
	respCode, res, err := http.Post(reqURL, body, opts...)
	if err != nil {
		return respCode, []byte{}, err
	}

	return respCode, res, nil
}
