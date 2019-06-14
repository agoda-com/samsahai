package agodacspider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/util/http"
)

const (
	CheckerName = "agoda-cspider"

	MaxRequestsTimeout   = 60 * time.Second
	MaxOneRequestTimeout = 10 * time.Second

	EnvURL         = "agoda-cspider-url"
	EnvAccessToken = "agoda-cspider-access-token"
)

var (
	ErrAccessTokenNotProvided = fmt.Errorf("access token not provided")
	ErrURLNotProvided         = fmt.Errorf("url not provided")
	ErrRequestTimeout         = fmt.Errorf("request timeout")
	ErrAppNameNotFound        = fmt.Errorf("appname not found")
)

type checker struct {
	log         logr.Logger
	accessToken string
	baseURL     string
	matcher     *regexp.Regexp
}

type cspiderReq struct {
	App  string `json:"app"`
	DC   string `json:"dc"`
	Role string `json:"role"`
}
type cspiderRes struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    string `json:"data"`
}
type cspiderTag struct {
	App     string `json:"app"`
	Server  string `json:"server"`
	DC      string `json:"dc"`
	Version string `json:"version"`
}

var internalServiceNames = map[string]string{
	"capi":               "customerapi",
	"contentplatform":    "Property API Content",
	"dfapi":              "Property API Pricing",
	"heisenberg":         "Heisenberg App",
	"hermes":             "Hermes API",
	"maxwell":            "Maxwell App",
	"propertyapi":        "Property API Search",
	"soybean":            "Soybean App",
	"txapi":              "TXAPI",
	"zenith":             "Zenith",
	"npcapi":             "NPC API",
	"ebe-api-v3":         "ebe_api_v3",
	"ebe-api-v2":         "ebe_api_v2",
	"ebe-payment-api":    "ebe_payment_api",
	"ebe-creditcard-api": "ebe_creditcard_service",
}

func New() internal.DesiredComponentChecker {
	c := &checker{
		log:         log.Log.WithName(CheckerName),
		accessToken: viper.GetString(EnvAccessToken),
		baseURL:     viper.GetString(EnvURL),
	}
	if c.baseURL == "" {
		c.log.Error(ErrURLNotProvided, "please specify url to cspider")
		return nil
	}
	if c.accessToken == "" {
		c.log.Error(ErrAccessTokenNotProvided, "please specify access token")
		return nil
	}
	c.matcher, _ = regexp.Compile(`(\d+)([._-]\d+)+`)
	return c
}

func (c *checker) GetName() string {
	return CheckerName
}

func (c *checker) GetVersion(repository, name, pattern string) (string, error) {
	versionCh := make(chan string)
	errCh := make(chan error)
	ctx, cancelFn := context.WithTimeout(context.Background(), MaxRequestsTimeout)
	defer cancelFn()

	var appName string
	if _, ok := internalServiceNames[name]; !ok {
		return "", ErrAppNameNotFound
	}
	appName = internalServiceNames[name]

	go func() {
		reqJSON := &cspiderReq{App: appName, DC: "HKG"}
		reqData, err := json.Marshal(reqJSON)
		if err != nil {
			c.log.Error(err, "cannot marshal request data", "data", reqData)
			errCh <- err
			return
		}

		opts := []http.Option{
			http.WithContext(ctx),
			http.WithSkipTLSVerify(),
			http.WithTimeout(MaxOneRequestTimeout),
			http.WithHeader("access_token", c.accessToken),
		}
		resData, err := http.Post(c.baseURL, reqData, opts...)
		if err != nil {
			c.log.Error(err, "POST request failed", "url", c.baseURL, "data", string(reqData))
			errCh <- err
			return
		}

		var resJSON cspiderRes
		if err := json.Unmarshal(resData, &resJSON); err != nil {
			c.log.Error(err, "cannot unmarshal response data", "data", string(resData))
			errCh <- err
			return
		}
		var tags []cspiderTag
		if err := json.Unmarshal([]byte(resJSON.Data), &tags); err != nil {
			c.log.Error(err, "cannot unmarshal data from cspider", "data", resJSON.Data)
			errCh <- err
			return
		}

		versionCh <- c.getMajorityVersion(tags)
	}()

	select {
	case <-ctx.Done():
		c.log.Error(internal.ErrRequestTimeout, fmt.Sprintf("checking took more than %v", MaxRequestsTimeout))
		return "", ErrRequestTimeout
	case err := <-errCh:
		return "", err
	case version := <-versionCh:
		if version == "" {
			return "", internal.ErrNoDesiredComponentVersion
		}
		return version, nil
	}
}

func (c *checker) getMajorityVersion(tags []cspiderTag) string {
	versions := map[string]int{}

	for _, tag := range tags {
		version := c.extractVersion(tag.Version)
		if version == "" {
			continue
		}
		if _, ok := versions[version]; ok {
			versions[version]++
		} else {
			versions[version] = 1
		}
	}
	if len(versions) == 0 {
		return ""
	}

	// find maximum deployed version
	countVersions := map[int][]string{}
	maxVal := 0
	for k, v := range versions {
		if v > maxVal {
			maxVal = v
		}
		if _, ok := countVersions[v]; ok {
			countVersions[v] = append(countVersions[v], k)
		} else {
			countVersions[v] = []string{k}
		}
	}

	if len(countVersions[maxVal]) == 1 {
		return countVersions[maxVal][0]
	}

	// Get latest version
	sort.Sort(internal.SortableVersion(countVersions[maxVal]))
	return countVersions[maxVal][len(countVersions[maxVal])-1]
}

func (c *checker) extractVersion(version string) string {
	matches := c.matcher.FindStringSubmatch(version)
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}
