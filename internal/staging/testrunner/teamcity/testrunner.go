package teamcity

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
)

var logger = s2hlog.Log.WithName(TestRunnerName)

const (
	TestRunnerName = "teamcity"

	maxRunnerTimeout      = 15 * time.Second
	maxHTTPRequestTimeout = 10 * time.Second

	baseAPIPath = "httpAuth/app/rest"

	buildFinished = "finished"
	statusSuccess = "success"

	EnvParamEnvType         = "env.S2H_ENV_TYPE"
	EnvParamNamespace       = "env.S2H_NAMESPACE"
	EnvParamVersion         = "env.S2H_VERSION"
	EnvParamTeam            = "env.S2H_TEAM"
	EnvParamGitCommit       = "env.S2H_GIT_COMMIT"
	EnvParamCompName        = "env.S2H_COMPONENT_NAME"
	EnvParamCompVersion     = "env.S2H_COMPONENT_VERSION"
	EnvParamQueueType       = "env.S2H_QUEUE_TYPE"
	ParamEnvType            = "s2hEnvType"
	ParamNamespace          = "s2hNamespace"
	ParamVersion            = "s2hVersion"
	ParamTeam               = "s2hTeam"
	ParamGitCommit          = "s2hGitCommit"
	ParamCompName           = "s2hComponentName"
	ParamCompVersion        = "s2hComponentVersion"
	ParamQueueType          = "s2hQueueType"
	ParamReverseEnvType     = "reverse.dep.*.s2hEnvType"
	ParamReverseNamespace   = "reverse.dep.*.s2hNamespace"
	ParamReverseVersion     = "reverse.dep.*.s2hVersion"
	ParamReverseTeam        = "reverse.dep.*.s2hTeam"
	ParamReverseGitCommit   = "reverse.dep.*.s2hGitCommit"
	ParamReverseCompName    = "reverse.dep.*.s2hComponentName"
	ParamReverseCompVersion = "reverse.dep.*.s2hComponentVersion"
	ParamReverseQueueType   = "reverse.dep.*.s2hQueueType"
)

type TriggerResponse struct {
	BuildID string `xml:"id,attr"`
}

type ResultResponse struct {
	State  string `xml:"state,attr"`
	Status string `xml:"status,attr"`
}

type testRunner struct {
	username string
	password string
	baseURL  string
	client   client.Client
}

type triggerBuildReq struct {
	BranchName string         `json:"branchName"`
	BuildType  buildTypeJSON  `json:"buildType"`
	Properties propertiesJSON `json:"properties"`
}
type buildTypeJSON struct {
	ID string `json:"id"`
}
type propertiesJSON struct {
	Properties []propertyJSON `json:"property"`
}
type propertyJSON struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// TODO: should read teamcity credentials from secret
// New creates a new teamcity test runner
func New(client client.Client, baseURL, username, password string) internal.StagingTestRunner {
	t := &testRunner{
		client:   client,
		username: username,
		password: password,
		baseURL:  baseURL,
	}

	return t
}

// GetName implements the staging testRunner GetName function
func (t *testRunner) GetName() string {
	return TestRunnerName
}

// Trigger implements the staging testRunner Trigger function
func (t *testRunner) Trigger(testConfig *internal.ConfigTestRunner, currentQueue *v1beta1.Queue) error {
	if testConfig == nil {
		return errors.Wrapf(s2herrors.ErrTestConfiigurationNotFound,
			"test configuration should not be nil. queue: %s", currentQueue.Name)
	}

	errCh := make(chan error, 1)
	ctx, cancelFn := context.WithTimeout(context.Background(), maxRunnerTimeout)
	defer cancelFn()

	go func() {
		apiURL := fmt.Sprintf("%s/%s/buildQueue", t.baseURL, baseAPIPath)
		teamName := currentQueue.Spec.TeamName
		reqJSON := &triggerBuildReq{
			BranchName: testConfig.Teamcity.Branch,
			BuildType: buildTypeJSON{
				ID: testConfig.Teamcity.BuildTypeID,
			},
			Properties: propertiesJSON{
				Properties: []propertyJSON{
					{Name: ParamEnvType, Value: currentQueue.GetEnvType()},
					{Name: EnvParamEnvType, Value: currentQueue.GetEnvType()},
					{Name: ParamReverseEnvType, Value: currentQueue.GetEnvType()},
					{Name: ParamNamespace, Value: currentQueue.Namespace},
					{Name: EnvParamNamespace, Value: currentQueue.Namespace},
					{Name: ParamReverseNamespace, Value: currentQueue.Namespace},
					{Name: ParamVersion, Value: internal.Version},
					{Name: EnvParamVersion, Value: internal.Version},
					{Name: ParamReverseVersion, Value: internal.Version},
					{Name: ParamTeam, Value: teamName},
					{Name: EnvParamTeam, Value: teamName},
					{Name: ParamReverseTeam, Value: teamName},
					{Name: ParamGitCommit, Value: internal.GitCommit},
					{Name: EnvParamGitCommit, Value: internal.GitCommit},
					{Name: ParamReverseGitCommit, Value: internal.GitCommit},
					{Name: ParamCompName, Value: currentQueue.Name},
					{Name: EnvParamCompName, Value: currentQueue.Name},
					{Name: ParamReverseCompName, Value: currentQueue.Name},
					{Name: ParamCompVersion, Value: currentQueue.Spec.Version},
					{Name: EnvParamCompVersion, Value: currentQueue.Spec.Version},
					{Name: ParamReverseCompVersion, Value: currentQueue.Spec.Version},
					{Name: ParamQueueType, Value: currentQueue.GetQueueType()},
					{Name: EnvParamQueueType, Value: currentQueue.GetQueueType()},
					{Name: ParamReverseQueueType, Value: currentQueue.GetQueueType()},
				},
			},
		}

		opts := []http.Option{
			http.WithSkipTLSVerify(),
			http.WithTimeout(maxHTTPRequestTimeout),
			http.WithBasicAuth(t.username, t.password),
		}
		reqBody, err := json.Marshal(reqJSON)
		if err != nil {
			logger.Error(err, "cannot marshal request data", "data", reqBody)
			errCh <- err
			return
		}

		resp, err := http.Post(apiURL, reqBody, opts...)
		if err != nil {
			logger.Error(err, "POST request failed", "url", apiURL, "data", string(reqBody))
			errCh <- err
			return
		}

		out := &TriggerResponse{}
		if err := xml.Unmarshal([]byte(resp), out); err != nil {
			logger.Error(err, "cannot unmarshal xml response data")
			errCh <- err
			return
		}

		// update build id / build type id / build url to queue status
		buildTypeID := testConfig.Teamcity.BuildTypeID
		buildURL := fmt.Sprintf("%s/viewLog.html?buildId=%s&buildTypeId=%s", t.baseURL, out.BuildID, testConfig.Teamcity.BuildTypeID)
		currentQueue.Status.TestRunner.Teamcity.SetTeamcity(out.BuildID, buildTypeID, buildURL)
		if err := t.client.Update(ctx, currentQueue); err != nil {
			errCh <- err
			return
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
func (t *testRunner) GetResult(testConfig *internal.ConfigTestRunner, currentQueue *v1beta1.Queue) (isResultSuccess bool, isBuildFinished bool, err error) {
	if testConfig == nil {
		return false, false, errors.Wrapf(s2herrors.ErrTestConfiigurationNotFound,
			"test configuration should not be nil. queue: %s", currentQueue.Name)
	}

	buildID := currentQueue.Status.TestRunner.Teamcity.BuildID
	buildTypeID := testConfig.Teamcity.BuildTypeID
	apiURL := fmt.Sprintf("%s/httpAuth/app/rest/builds/id:%s?locator=buildType:%s", t.baseURL, buildID, buildTypeID)
	opts := []http.Option{
		http.WithSkipTLSVerify(),
		http.WithBasicAuth(t.username, t.password),
	}

	resp, err := http.Get(apiURL, opts...)
	if err != nil {
		logger.Error(err, "The HTTP request failed")
		return false, false, err
	}

	var byteData = []byte(resp)
	var response ResultResponse
	err = xml.Unmarshal(byteData, &response)
	if err != nil {
		logger.Error(err, "cannot unmarshal request data")
		return false, false, err
	}

	isBuildFinished = false
	if strings.EqualFold(buildFinished, response.State) {
		isBuildFinished = true
	}

	isResultSuccess = false
	if strings.EqualFold(statusSuccess, response.Status) {
		isResultSuccess = true
	}

	return isResultSuccess, isBuildFinished, nil
}
