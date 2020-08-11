package helm3

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ghodss/yaml"
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
	DefaultUninstallTimeout = 300 * time.Second
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
	prevNs := os.Getenv("HELM_NAMESPACE")
	_ = os.Setenv("HELM_NAMESPACE", ns)
	settings := cli.New()
	_ = os.Setenv("HELM_NAMESPACE", prevNs)

	e := engine{
		namespace:      ns,
		debug:          debug,
		actionSettings: new(action.Configuration),
		settings:       settings,
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
	_ *v1beta1.Component,
	parentComp *v1beta1.Component,
	values map[string]interface{},
	deployTimeout *time.Duration,
) error {
	if err := e.helmInit(); err != nil {
		return err
	}

	cpo := action.ChartPathOptions{
		Version: parentComp.Chart.Version,
		RepoURL: parentComp.Chart.Repository,
	}

	_, err := e.GetHistories(refName)
	if err != nil {
		switch err {
		case driver.ErrReleaseNotFound:
			err = e.helmInstall(refName, parentComp.Chart.Name, cpo, values, deployTimeout)
			if err != nil {
				return err
			}
			return nil
		default:
			return errors.Wrapf(err, "cannot get history of release %q", refName)
		}
	}

	// update
	err = e.helmUpgrade(refName, parentComp.Chart.Name, cpo, values, deployTimeout)
	if err != nil {
		return err
	}

	return nil
}

func (e *engine) Rollback(refName string, revision int) error {
	return e.helmRollback(refName, revision)
}

func (e *engine) GetHistories(refName string) ([]*release.Release, error) {
	cliHist := action.NewHistory(e.actionSettings)
	cliHist.Max = 1

	return cliHist.Run(refName)
}

func (e *engine) Delete(refName string) error {
	if err := e.helmInit(); err != nil {
		return err
	}

	releaseName, err := e.ensureReleaseName(refName)
	if err != nil {
		return err
	}

	if releaseName == "" {
		return nil
	}

	logger.Debug("deleting release", "releaseName", releaseName)
	if err := e.helmUninstall(releaseName, false); err != nil {
		return err
	}

	return nil
}

func (e *engine) ForceDelete(refName string) error {
	releaseName, err := e.ensureReleaseName(refName)
	if err != nil {
		return err
	}

	if releaseName == "" {
		return nil
	}

	// delete release
	if err := e.helmUninstall(releaseName, true); err != nil {
		return err
	}

	return nil
}

func (e *engine) GetValues() (map[string][]byte, error) {
	valuesYaml, err := e.helmGetValues()
	if err != nil {
		return nil, err
	}

	return valuesYaml, nil
}

func (e *engine) GetReleases() ([]*release.Release, error) {
	releases, err := e.helmList()
	if err != nil {
		return []*release.Release{}, err
	}

	return releases, nil
}

func (e *engine) helmUninstall(refName string, disableHooks bool) error {
	client := action.NewUninstall(e.actionSettings)
	client.Timeout = DefaultUninstallTimeout
	client.DisableVerify = true
	client.DisableHooks = disableHooks

	logger.Debug("deleting release", "refName", refName)
	if _, err := client.Run(refName); err != nil {
		switch {
		case errors.Is(errors.Cause(err), driver.ErrReleaseNotFound): // nolint
			return nil
		}
		return errors.Wrap(err, "error while deleting helm release")
	}

	return nil
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
	deployTimeout *time.Duration,
) error {
	logger.Debug("helm install", "releaseName", refName, "chartName", chartName)

	client := action.NewInstall(e.actionSettings)
	client.ChartPathOptions = cpo
	client.Namespace = e.namespace
	client.ReleaseName = refName
	client.DisableVerify = true
	if deployTimeout != nil {
		client.Timeout = *deployTimeout
		client.Wait = true
	}

	ch, err := e.helmPrepareChart(chartName, cpo)
	if err != nil {
		logger.Error(err, "helm prepare chart failed", "releaseName", refName, "chartName", chartName)
		return err
	}
	logger.Debug("helm prepare chart", "releaseName", refName, "chartName", chartName)

	_, err = client.Run(ch, values)
	if err != nil {
		logger.Error(err, "helm install failed", "releaseName", refName, "chartName", chartName)
		return errors.Wrapf(err, "helm install failed")
	}
	logger.Debug("helm install completed", "releaseName", refName, "chartName", chartName)

	return nil
}

func (e *engine) helmUpgrade(
	refName string,
	chartName string,
	cpo action.ChartPathOptions,
	values map[string]interface{},
	deployTimeout *time.Duration,
) error {
	logger.Debug("helm upgrade", "releaseName", refName, "chartName", chartName)

	client := action.NewUpgrade(e.actionSettings)
	client.ChartPathOptions = cpo
	client.Namespace = e.namespace
	client.Atomic = true
	client.DisableVerify = true
	if deployTimeout != nil {
		client.Timeout = *deployTimeout
		client.Wait = true
	}

	ch, err := e.helmPrepareChart(chartName, cpo)
	if err != nil {
		logger.Error(err, "helm prepare chart failed", "releaseName", refName, "chartName", chartName)
		return err
	}

	_, err = client.Run(refName, ch, values)
	if err != nil {
		logger.Error(err, "helm upgrade failed", "releaseName", refName, "chartName", chartName)
		return errors.Wrapf(err, "helm upgrade failed")
	}

	return nil
}

func (e *engine) helmRollback(refName string, revision int) error {
	logger.Debug("helm rollback", "releaseName", refName, "revision", revision)

	client := action.NewRollback(e.actionSettings)
	client.Version = revision

	err := client.Run(refName)
	if err != nil {
		logger.Error(err, "helm rollback failed", "releaseName", refName, "revision", revision)
		return errors.Wrapf(err, "helm rollback failed")
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

func (e *engine) helmList() ([]*release.Release, error) {
	client := action.NewList(e.actionSettings)
	client.StateMask = action.ListAll
	client.All = true
	releases, err := client.Run()
	if err != nil {
		return nil, err
	}
	return releases, nil
}

func (e *engine) helmGetValues() (map[string][]byte, error) {
	releases, err := e.helmList()
	if err != nil {
		return nil, err
	}

	valuesYaml := make(map[string][]byte)
	for _, r := range releases {
		client := action.NewGetValues(e.actionSettings)
		values, err := client.Run(r.Name)
		if err != nil {
			return nil, err
		}

		valuesData, err := json.Marshal(values)
		if err != nil {
			return nil, err
		}

		yml, err := yaml.JSONToYAML(valuesData)
		if err != nil {
			return nil, err
		}

		valuesYaml[r.Name] = yml
	}

	return valuesYaml, nil
}

func (e *engine) ensureReleaseName(refName string) (string, error) {
	releases, err := e.helmList()
	if err != nil {
		return "", err
	}

	for _, r := range releases {
		if strings.Contains(r.Name, refName) {
			return r.Name, nil
		}
	}

	return "", nil
}

// DeleteAllReleases deletes all releases in the namespace
func DeleteAllReleases(ns string, debug bool) error {
	e := New(ns, debug).(*engine)

	releases, err := e.helmList()
	if err != nil {
		return err
	}
	for _, r := range releases {
		err := e.Delete(r.Name)
		if err != nil {
			return err
		}
	}
	return nil
}
