package rest

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/internal/util/http"
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
	s2hv1beta1.Image
}

type pullRequestTriggerRest struct {
	ReporterJSON
	internal.PullRequestTriggerReporter
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

// WithRestClient specifies rest client to override when creating rest reporter
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

// SendComponentUpgrade send details of component upgrade via http POST
func (r *reporter) SendComponentUpgrade(configCtrl internal.ConfigController, comp *internal.ComponentUpgradeReporter) error {
	config, err := configCtrl.Get(comp.TeamName)
	if err != nil {
		return err
	}

	if config.Spec.Reporter == nil ||
		config.Spec.Reporter.Rest == nil ||
		config.Spec.Reporter.Rest.ComponentUpgrade == nil {
		return nil
	}

	for _, ep := range config.Spec.Reporter.Rest.ComponentUpgrade.Endpoints {
		restObj := &componentUpgradeRest{NewReporterJSON(), *comp}
		body, err := json.Marshal(restObj)
		if err != nil {
			logger.Error(err, fmt.Sprintf("cannot convert struct to json object, %v", body))
			return err
		}

		if err = r.send(ep.URL, body, internal.ComponentUpgradeType); err != nil {
			return err
		}
	}

	return nil
}

// SendActivePromotionStatus send active promotion status via http POST
func (r *reporter) SendActivePromotionStatus(configCtrl internal.ConfigController, atpRpt *internal.ActivePromotionReporter) error {
	config, err := configCtrl.Get(atpRpt.TeamName)
	if err != nil {
		return err
	}

	if config.Spec.Reporter == nil ||
		config.Spec.Reporter.Rest == nil ||
		config.Spec.Reporter.Rest.ActivePromotion == nil {
		return nil
	}

	for _, ep := range config.Spec.Reporter.Rest.ActivePromotion.Endpoints {
		restObj := &activePromotionRest{NewReporterJSON(), *atpRpt}
		body, err := json.Marshal(restObj)
		if err != nil {
			logger.Error(err, fmt.Sprintf("cannot convert struct to json object, %v", body))
			return err
		}

		if err = r.send(ep.URL, body, internal.ActivePromotionType); err != nil {
			return err
		}
	}

	return nil
}

// SendImageMissing implements the reporter SendImageMissing function
func (r *reporter) SendImageMissing(configCtrl internal.ConfigController, imageMissingRpt *internal.ImageMissingReporter) error {
	config, err := configCtrl.Get(imageMissingRpt.TeamName)
	if err != nil {
		return err
	}

	if config.Spec.Reporter == nil ||
		config.Spec.Reporter.Rest == nil ||
		config.Spec.Reporter.Rest.ImageMissing == nil {
		return nil
	}

	for _, ep := range config.Spec.Reporter.Rest.ImageMissing.Endpoints {
		restObj := &imageMissingRest{NewReporterJSON(), imageMissingRpt.Image}
		body, err := json.Marshal(restObj)
		if err != nil {
			logger.Error(err, fmt.Sprintf("cannot convert struct to json object, %v", body))
			return err
		}

		if err = r.send(ep.URL, body, internal.ImageMissingType); err != nil {
			return err
		}
	}

	return nil
}

// SendPullRequestTriggerResult implements the reporter SendPullRequestTriggerResult function
func (r *reporter) SendPullRequestTriggerResult(configCtrl internal.ConfigController, prTriggerRpt *internal.PullRequestTriggerReporter) error {
	config, err := configCtrl.Get(prTriggerRpt.TeamName)
	if err != nil {
		return err
	}

	if config.Spec.Reporter == nil ||
		config.Spec.Reporter.Rest == nil ||
		config.Spec.Reporter.Rest.PullRequestTrigger == nil {
		return nil
	}

	for _, ep := range config.Spec.Reporter.Rest.PullRequestTrigger.Endpoints {
		restObj := &pullRequestTriggerRest{NewReporterJSON(), *prTriggerRpt}
		body, err := json.Marshal(restObj)
		if err != nil {
			logger.Error(err, fmt.Sprintf("cannot convert struct to json object, %v", body))
			return err
		}

		if err = r.send(ep.URL, body, internal.PullRequestTriggerType); err != nil {
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
	// TODO: duplicate get/post
	if _, _, err := restCli.Post("/", body); err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot send request to %s", restCli.BaseURL))
	}

	return nil
}

func generateUUID() string {
	return uuid.New().String()
}
