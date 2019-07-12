package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
	"github.com/agoda-com/samsahai/internal/util/template"
	"github.com/agoda-com/samsahai/pkg/samsahai/rpc"
)

var logger = s2hlog.Log.WithName(ReporterName)

const (
	ReporterName = "rest"
)

type reporter struct {
	rest *http.Client
}

// ReporterJSON represents generic json data for http POST report
type ReporterJSON struct {
	UnixTimestamp int64  `json:"unixTimestamp,omitempty"`
	UUID          string `json:"uuid,omitempty"`
}

type componentUpgradeRest struct {
	ReporterJSON
	internal.ComponentUpgradeReporter
}

type activePromotionRest struct {
	ReporterJSON
	internal.ActivePromotionReporter
}

type imageMissingRest struct {
	ReporterJSON
	rpc.Image
}

// NewReporterJSON creates new reporter json
func NewReporterJSON() ReporterJSON {
	unixTimestamp := time.Now().UnixNano()
	return ReporterJSON{
		UnixTimestamp: unixTimestamp,
		UUID:          generateUUID(),
	}
}

// NewOption allows specifying various configuration
type NewOption func(*reporter)

// WithRestClient specifies rest client to override when create rest reporter
func WithRestClient(rest *http.Client) NewOption {
	if rest == nil {
		panic("Rest client should not be nil")
	}

	return func(r *reporter) {
		r.rest = rest
	}
}

// New creates a new rest reporter
func New(opts ...NewOption) internal.Reporter {
	r := &reporter{}
	// apply the new options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// NewRest returns reporter for sending report to http rest
func NewRest(url string) *http.Client {
	return http.NewClient(url)
}

// GetName returns rest type
func (r *reporter) GetName() string {
	return ReporterName
}

// SendComponentUpgrade send details of component upgrade failure via http POST
func (r *reporter) SendComponentUpgrade(configMgr internal.ConfigManager, comp *internal.ComponentUpgradeReporter) error {
	cfg := configMgr.Get()
	if cfg.Reporter == nil || cfg.Reporter.Rest == nil || cfg.Reporter.Rest.ComponentUpgrade == nil {
		return nil
	}

	configPath := configMgr.GetGitConfigPath()
	var err error
	var body string
	for _, ep := range cfg.Reporter.Rest.ComponentUpgrade.Endpoints {
		restObj := &componentUpgradeRest{NewReporterJSON(), *comp}
		body = r.renderBodyFromTemplate(configPath, ep.Template, restObj)
		if err = r.send(ep.URL, []byte(body), internal.ComponentUpgradeType); err != nil {
			return err
		}
	}

	return nil
}

// SendActivePromotionStatus send active promotion status via http POST
func (r *reporter) SendActivePromotionStatus(configMgr internal.ConfigManager, atpRpt *internal.ActivePromotionReporter) error {
	cfg := configMgr.Get()
	if cfg.Reporter == nil || cfg.Reporter.Rest == nil || cfg.Reporter.Rest.ActivePromotion == nil {
		return nil
	}

	configPath := configMgr.GetGitConfigPath()
	var err error
	var body string
	for _, ep := range cfg.Reporter.Rest.ActivePromotion.Endpoints {
		restObj := &activePromotionRest{NewReporterJSON(), *atpRpt}
		body = r.renderBodyFromTemplate(configPath, ep.Template, restObj)
		if err = r.send(ep.URL, []byte(body), internal.ActivePromotionType); err != nil {
			return err
		}
	}

	return nil
}

// SendImageMissing implements the reporter SendImageMissing function
func (r *reporter) SendImageMissing(configMgr internal.ConfigManager, img *rpc.Image) error {
	cfg := configMgr.Get()
	if cfg.Reporter == nil || cfg.Reporter.Rest == nil || cfg.Reporter.Rest.ImageMissing == nil {
		return nil
	}

	configPath := configMgr.GetGitConfigPath()
	var err error
	var body string
	for _, ep := range cfg.Reporter.Rest.ImageMissing.Endpoints {
		restObj := &imageMissingRest{NewReporterJSON(), *img}
		body = r.renderBodyFromTemplate(configPath, ep.Template, restObj)
		if err = r.send(ep.URL, []byte(body), internal.ImageMissingType); err != nil {
			return err
		}
	}

	return nil
}

// send provides handling convert ReporterJSON to []byte and sent it via http POST
func (r *reporter) send(url string, body []byte, event internal.EventType) error {
	restCli := r.rest
	if r.rest == nil {
		restCli = NewRest(url)
	}

	logger.Debug("start sending data via http POST", "event", event, "url", url)
	if _, err := restCli.Post("/", body); err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot send request to %s", restCli.BaseURL))
	}

	return nil
}

func (r *reporter) loadTemplate(path string) ([]byte, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return []byte{}, err
	}

	return data, nil
}

func (r *reporter) renderBodyFromTemplate(configPath, tplPath string, dataObj interface{}) string {
	if tplPath == "" {
		body, err := json.Marshal(dataObj)
		if err != nil {
			logger.Error(err, fmt.Sprintf("cannot convert struct to json object, %v", dataObj))
			return ""
		}

		return string(body)
	}

	path := filepath.Join(configPath, tplPath)
	tpl, err := r.loadTemplate(path)
	if err != nil {
		logger.Error(err, fmt.Sprintf("cannot load template from %s", path))
	}

	return template.TextRender("rest", string(tpl), dataObj)
}

func generateUUID() string {
	return uuid.New().String()
}
