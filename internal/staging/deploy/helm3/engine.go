package helm3

import (
	"fmt"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"

	"github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	"github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
)

var logger = s2hlog.Log.WithName(EngineName)

const (
	EngineName              = "helm3"
	HelmDriver              = "secrets"
	DefaultUninstallTimeout = 60 * time.Second
)

type engine struct {
	namespace      string
	debug          bool
	actionSettings *action.Configuration
	settings       *cli.EnvSettings
	helmDriver     string
	initLock       sync.Mutex
	initDone       uint32
}

func New(ns string, debug bool) internal.DeployEngine {
	e := engine{
		namespace:      ns,
		debug:          debug,
		actionSettings: new(action.Configuration),
		settings:       cli.New(),
		helmDriver:     HelmDriver,
	}
	err := e.helmInit()
	if err != nil {
		logger.Error(err, "error while initializing helm")
	}

	return &e
}

func (e *engine) printDebug(format string, args ...interface{}) {
	if e.debug {
		logger.Debug(fmt.Sprintf(format, args...))
	}
}

func (e *engine) GetName() string {
	return EngineName
}

func (e *engine) GetLabelSelectors(refName string) map[string]string {
	return map[string]string{"release": refName}
}

func (e *engine) IsMocked() bool {
	return false
}

func (e *engine) Create(
	refName string,
	_ *internal.Component,
	parentComp *internal.Component,
	values map[string]interface{},
) error {
	if err := e.helmInit(); err != nil {
		return err
	}

	cpo := action.ChartPathOptions{
		Version: parentComp.Chart.Version,
		RepoURL: parentComp.Chart.Repository,
	}

	cliHist := action.NewHistory(e.actionSettings)
	cliHist.Max = 1
	_, err := cliHist.Run(refName)
	if err != nil {
		switch err {
		case driver.ErrReleaseNotFound:
			err = e.helmInstall(refName, parentComp.Chart.Name, cpo, values)
			if err != nil {
				return err
			}
			return nil
		default:
			return errors.Wrapf(err, "cannot get history of release %q", refName)
		}
	}

	cliGet := action.NewGetValues(e.actionSettings)
	v, err := cliGet.Run(refName)
	if err != nil {
		return errors.Wrapf(err, "cannot get helm values of release %q", refName)
	}

	if !reflect.DeepEqual(values, v) {
		// update
		err = e.helmUpgrade(refName, parentComp.Chart.Name, cpo, values)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *engine) Delete(refName string) error {
	if refName == "" {
		return nil
	}

	if err := e.helmInit(); err != nil {
		return err
	}

	cliUninstall := action.NewUninstall(e.actionSettings)
	cliUninstall.Timeout = DefaultUninstallTimeout

	logger.Debug("deleting release", "releaseName", refName)
	_, err := cliUninstall.Run(refName)
	if err != nil {
		switch {
		case errors.Is(errors.Cause(err), driver.ErrReleaseNotFound):
			return nil
		}
		return errors.Wrap(err, "error while deleting helm release")
	}

	return nil
}

func (e *engine) IsReady(queue *v1beta1.Queue) (bool, error) {
	refName := queue.Status.ReleaseName

	client := action.NewStatus(e.actionSettings)
	rel, err := client.Run(refName)
	if err != nil {
		return false, errors.Wrap(err, "cannot get status of helm release")
	}

	return rel.Info.Status == release.StatusDeployed, nil
}

func (e *engine) helmInit() error {
	if atomic.LoadUint32(&e.initDone) == 1 {
		return nil
	}

	e.initLock.Lock()
	defer e.initLock.Unlock()

	if e.initDone == 0 {
		err := e.actionSettings.Init(e.settings.RESTClientGetter(), e.namespace, e.helmDriver, e.printDebug)
		if err != nil {
			return errors.Wrap(err, "cannot init helm action settings")
		}
		atomic.StoreUint32(&e.initDone, 1)
	}

	return nil
}

func (e *engine) helmInstall(
	refName string,
	chartName string,
	cpo action.ChartPathOptions,
	values map[string]interface{},
) error {
	logger.Debug("helm install", "releaseName", refName, "chartName", chartName)

	client := action.NewInstall(e.actionSettings)
	client.ChartPathOptions = cpo
	client.Namespace = e.namespace
	client.ReleaseName = refName

	ch, err := e.helmPrepareChart(chartName, cpo)
	if err != nil {
		return err
	}

	_, err = client.Run(ch, values)
	if err != nil {
		return errors.Wrapf(err, "helm install failed")
	}

	return nil
}

func (e *engine) helmUpgrade(
	refName string,
	chartName string,
	cpo action.ChartPathOptions,
	values map[string]interface{},
) error {
	logger.Debug("helm upgrade", "releaseName", refName, "chartName", chartName)

	client := action.NewUpgrade(e.actionSettings)
	client.ChartPathOptions = cpo
	client.Namespace = e.namespace
	client.Atomic = true

	ch, err := e.helmPrepareChart(chartName, cpo)
	if err != nil {
		return err
	}

	_, err = client.Run(refName, ch, values)
	if err != nil {
		return errors.Wrapf(err, "helm upgrade failed")
	}

	return nil
}

func (e *engine) helmPrepareChart(
	chartName string,
	cpo action.ChartPathOptions,
) (*chart.Chart, error) {
	cp, err := cpo.LocateChart(chartName, e.settings)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot locate chart: %s", chartName)
	}

	// Check chart dependencies to make sure all are present in /charts
	ch, err := loader.Load(cp)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot load chart: %s", chartName)
	}

	switch ch.Metadata.Type {
	case "", "application":
	default:
		return nil, errors.Wrapf(err, "%s chart type not supported", ch.Metadata.Type)
	}

	p := getter.All(e.settings)

	if req := ch.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(ch, req); err != nil {
			man := &downloader.Manager{
				Out:              os.Stdout,
				ChartPath:        cp,
				Keyring:          cpo.Keyring,
				SkipUpdate:       false,
				Getters:          p,
				RepositoryConfig: e.settings.RepositoryConfig,
				RepositoryCache:  e.settings.RepositoryCache,
			}
			if err := man.Update(); err != nil {
				return nil, errors.Wrapf(err, "helm download dependency charts failed")
			}
		}
	}

	if ch.Metadata.Deprecated {
		logger.Warnf("%s chart is deprecated", chartName)
	}

	return ch, nil
}
