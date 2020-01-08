package fluxhelm

import (
	"reflect"

	fluxv1beta1 "github.com/fluxcd/flux/integrations/apis/flux.weave.works/v1beta1"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/agoda-com/samsahai/internal"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	"github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

var logger = s2hlog.Log.WithName(EngineName)

type Action string

const (
	EngineName = "flux-helm"

	InstallAction Action = "install"
	UpgradeAction Action = "upgrade"
)

type engine struct {
	configMgr              internal.ConfigManager
	hrClient               internal.HelmReleaseClient
	lastAction             Action
	lastObservedGeneration int64
}

// New creates a new teamcity test runner
func New(configMgr internal.ConfigManager, hrClient internal.HelmReleaseClient) internal.DeployEngine {
	return &engine{
		configMgr: configMgr,
		hrClient:  hrClient,
	}
}

func (e *engine) GetName() string {
	return EngineName
}

func (e *engine) Create(
	refName string,
	_ *internal.Component,
	parentComp *internal.Component,
	values map[string]interface{},
) error {
	hr := fluxv1beta1.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name: refName,
		},
		Spec: fluxv1beta1.HelmReleaseSpec{
			ReleaseName: refName,
			ChartSource: fluxv1beta1.ChartSource{
				RepoChartSource: &fluxv1beta1.RepoChartSource{
					Name:    parentComp.Chart.Name,
					RepoURL: parentComp.Chart.Repository,
					Version: parentComp.Chart.Version,
				},
			},
			HelmValues: fluxv1beta1.HelmValues{Values: values},
		},
	}

	fetched, err := e.hrClient.Get(refName, metav1.GetOptions{})
	if err != nil && k8serrors.IsNotFound(err) {
		// deploy parent component chart
		_, err := e.hrClient.Create(&hr)
		e.lastAction = InstallAction
		return errors.Wrap(err, "cannot create HelmRelease")
	} else if err != nil {
		return err
	}

	// found, try to update if spec is changed
	if !reflect.DeepEqual(fetched.Spec, hr.Spec) {
		// update
		fetched.Spec = hr.Spec
		_, err := e.hrClient.Update(fetched)
		e.lastAction = UpgradeAction
		e.lastObservedGeneration = hr.Status.ObservedGeneration
		return errors.Wrap(err, "cannot update HelmRelease")
	}

	//logger.Debug(fmt.Sprintf("create env with resource key: %s", resourceKey))
	return nil
}

func (e *engine) Delete(queue *v1beta1.Queue) error {
	list, err := e.hrClient.List(metav1.ListOptions{})
	if err != nil {
		logger.Error(err, "list helmrelease error")
		return err
	}

	for _, hr := range list.Items {
		if err := e.hrClient.Delete(hr.Name, nil); err != nil {
			return err
		}
	}

	// TODO: should we apply helm fast delete (delete pods with grace period)

	// TODO: wait for namespace to be empty before return nil

	return nil
}

func (e *engine) IsReady(queue *v1beta1.Queue) (bool, error) {
	// check helm release exist
	hr, err := e.hrClient.Get(queue.Status.ReleaseName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	switch e.lastAction {
	case UpgradeAction:
		if e.lastObservedGeneration == hr.Status.ObservedGeneration {
			return false, nil
		}
	default:
		if hr.Status.ReleaseStatus != "DEPLOYED" {
			return false, nil
		}
	}

	return true, nil
}

func (e *engine) GetLabelSelectors(refName string) map[string]string {
	return map[string]string{"release": refName}
}

func (e *engine) IsMocked() bool {
	return false
}
