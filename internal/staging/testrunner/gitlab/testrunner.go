package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	s2hv1 "github.com/agoda-com/samsahai/api/v1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/template"
)

var logger = s2hlog.Log.WithName(TestRunnerName)

const (
	TestRunnerName = "gitlab"

	maxRunnerTimeout      = 15 * time.Second
	maxHTTPRequestTimeout = 10 * time.Second

	baseAPIPath = "api/v4/projects"

	statusSuccess = "success"

	ParamEnvType     = "s2hEnvType"
	ParamNamespace   = "s2hNamespace"
	ParamVersion     = "s2hVersion"
	ParamTeam        = "s2hTeam"
	ParamGitCommit   = "s2hGitCommit"
	ParamCompName    = "s2hComponentName"
	ParamCompVersion = "s2hComponentVersion"
	ParamQueueType   = "s2hQueueType"
)

type TriggerResponse struct {
	ID     int    `json:"id"`
	WebURL string `json:"web_url"`
}

type ResultResponse struct {
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Status     string `json:"status"`
}

type testRunner struct {
	baseURL      string
	privateToken string
	client       client.Client
}

type triggerBuildReq struct {
	Variables map[string]string `json:"variables"`
	Token     string            `json:"token"`
}

// NewOption allows specifying various configuration
type NewOption func(*testRunner)

// WithGitlabToken specifies a gitlab private token to override when creating Gitlab test runner
// This will be used to get test result from the pipeline
func WithGitlabToken(token string) NewOption {
	return func(r *testRunner) {
		r.privateToken = token
	}
}

// New creates a new gitlab test runner
func New(client client.Client, baseURL string, opts ...NewOption) internal.StagingTestRunner {
	t := &testRunner{
		client:  client,
		baseURL: baseURL,
	}

	// apply the new options
	for _, opt := range opts {
		opt(t)
	}

	return t
}

// GetName implements the staging testRunner GetName function
func (t *testRunner) GetName() string {
	return TestRunnerName
}

// Trigger implements the staging testRunner Trigger function
func (t *testRunner) Trigger(testConfig *s2hv1.ConfigTestRunner, currentQueue *s2hv1.Queue) error {
	if testConfig == nil {
		return errors.Wrapf(s2herrors.ErrTestConfigurationNotFound,
			"test configuration should not be nil. queue: %s", currentQueue.Name)
	}

	projectID := testConfig.Gitlab.ProjectID
	pipelineTriggerToken := testConfig.Gitlab.PipelineTriggerToken
	branchName := testConfig.Gitlab.Branch
	prData := internal.PullRequestData{PRNumber: currentQueue.Spec.PRNumber}
	if prData.PRNumber != "" {
		branchName = template.TextRender("PullRequestBranchName", branchName, prData)
	}

	if branchName == "" {
		branchName = testConfig.Gitlab.Branch
	}

	errCh := make(chan error, 1)
	ctx, cancelFn := context.WithTimeout(context.Background(), maxRunnerTimeout)
	defer cancelFn()

	go func() {
		apiURL := fmt.Sprintf("%s/%s/%s/ref/%s/trigger/pipeline", t.baseURL, baseAPIPath, projectID, branchName)
		teamName := currentQueue.Spec.TeamName
		compVersion := "multiple-components"
		if len(currentQueue.Spec.Components) == 1 {
			compVersion = currentQueue.Spec.Components[0].Version
		}
		reqJSON := &triggerBuildReq{
			Token: pipelineTriggerToken,
			Variables: map[string]string{
				ParamEnvType:     currentQueue.GetEnvType(),
				ParamNamespace:   currentQueue.Namespace,
				ParamVersion:     internal.Version,
				ParamTeam:        teamName,
				ParamGitCommit:   internal.GitCommit,
				ParamCompName:    currentQueue.Name,
				ParamCompVersion: compVersion,
				ParamQueueType:   currentQueue.GetQueueType(),
			},
		}

		opts := []http.Option{
			http.WithSkipTLSVerify(),
			http.WithTimeout(maxHTTPRequestTimeout),
		}
		reqBody, err := json.Marshal(reqJSON)
		if err != nil {
			logger.Error(err, "cannot marshal request data", "data", reqBody)
			errCh <- err
			return
		}

		_, resp, err := http.Post(apiURL, reqBody, opts...)
		if err != nil {
			logger.Error(err, "POST request failed", "url", apiURL, "data", string(reqBody))
			errCh <- err
			return
		}

		out := &TriggerResponse{}
		if err := json.Unmarshal(resp, out); err != nil {
			logger.Error(err, "cannot unmarshal json response data")
			errCh <- err
			return
		}

		currentQueue.Status.TestRunner.Gitlab.SetGitlab(branchName, strconv.Itoa(out.ID), out.WebURL,
			fmt.Sprintf("#%d", out.ID))
		if t.client != nil {
			if err := t.client.Update(ctx, currentQueue); err != nil {
				errCh <- err
				return
			}
		}

		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Error(s2herrors.ErrRequestTimeout, fmt.Sprintf("triggering took more than %v", maxRunnerTimeout))
		return s2herrors.ErrRequestTimeout
	case err := <-errCh:
		return err
	}
}

// GetResult implements the staging testRunner GetResult function
func (t *testRunner) GetResult(testConfig *s2hv1.ConfigTestRunner, currentQueue *s2hv1.Queue) (
	isResultSuccess bool, isBuildFinished bool, err error) {

	if testConfig == nil {
		return false, true, errors.Wrapf(s2herrors.ErrTestConfigurationNotFound,
			"test configuration should not be nil. queue: %s", currentQueue.Name)
	}

	projectID := testConfig.Gitlab.ProjectID
	pipelineID := currentQueue.Status.TestRunner.Gitlab.PipelineID

	if projectID == "" || pipelineID == "" {
		return false, true, errors.Wrapf(s2herrors.ErrTestIDEmpty,
			"cannot get test result. projectID: '%s'. pipelineID: '%s'. queue: %s", projectID, pipelineID, currentQueue.Name)
	}

	apiURL := fmt.Sprintf("%s/%s/%s/pipelines/%s", t.baseURL, baseAPIPath, projectID, pipelineID)
	opts := []http.Option{
		http.WithSkipTLSVerify(),
	}

	if t.privateToken != "" {
		opts = append(opts, http.WithHeader("PRIVATE-TOKEN", t.privateToken))
	}

	_, resp, err := http.Get(apiURL, opts...)
	if err != nil {
		logger.Error(err, "The HTTP request failed", "URL", apiURL)
		return false, false, err
	}

	var byteData = resp
	var response ResultResponse
	err = json.Unmarshal(byteData, &response)
	if err != nil {
		logger.Error(err, "cannot unmarshal request data")
		return false, false, err
	}

	isBuildFinished = !strings.EqualFold("", response.StartedAt) && !strings.EqualFold("", response.FinishedAt)

	return strings.EqualFold(statusSuccess, response.Status), isBuildFinished, nil
}
