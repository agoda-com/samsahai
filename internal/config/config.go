package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/ghodss/yaml"
	"github.com/jvsteiner/multilock"
	"gopkg.in/src-d/go-git.v4"

	"github.com/agoda-com/samsahai/internal"
	s2hgit "github.com/agoda-com/samsahai/internal/config/git"
	s2herrors "github.com/agoda-com/samsahai/internal/errors"
	s2hlog "github.com/agoda-com/samsahai/internal/log"
	s2hv1beta1 "github.com/agoda-com/samsahai/pkg/apis/env/v1beta1"
)

type processWithError struct {
	processing bool
	err        error
}

var (
	logger    = s2hlog.Log.WithName(ManagerName)
	isCloning = make(map[string]processWithError)
	isPulling = make(map[string]processWithError)

	multiLock = multilock.NewMultiLock()
)

const (
	ManagerName    = "config-manager"
	ConfigFileName = "samsahai.yaml"
	EnvsDir        = "envs"
	//ComponentsDir  = "components"

	//EnvDirStaging   = internal.EnvStaging
	//EnvDirPreActive = internal.EnvPreActive
	//EnvDirActive    = internal.EnvActive
)

type manager struct {
	teamName        string
	configPath      string
	config          *internal.Configuration
	gitClient       *s2hgit.Client
	gitHeadRevision string
}

// NewWithBytes creates config manager with defined data bytes
func NewWithBytes(data []byte) internal.ConfigManager {
	m := manager{}
	err := m.load(data)
	if err != nil {
		logger.Error(err, "cannot load bytes")
		return nil
	}
	return &m
}

// NewWithGit creates config manager with defined git storage
func NewWithGit(teamName string, gitStorage s2hv1beta1.GitStorage, gitCred *s2hv1beta1.UsernamePasswordCredential) (internal.ConfigManager, error) {
	opts := addGitOptions(gitStorage, gitCred)
	gitClient, err := s2hgit.NewClient(teamName, gitStorage.URL, opts...)
	if err != nil {
		return nil, err
	}

	if err := gitClone(gitClient, teamName); err != nil {
		return nil, err
	}

	return NewWithGitClient(gitClient, teamName, path.Join(gitClient.GetDirectoryPath(), gitStorage.Path))
}

// NewWithGitClient creates config manager with defined git client and config path
func NewWithGitClient(client *s2hgit.Client, teamName, configPath string) (internal.ConfigManager, error) {
	p := configPath
	if !path.IsAbs(configPath) {
		pwd, _ := os.Getwd()
		p = path.Join(pwd, configPath)
	}

	m := manager{
		teamName:   teamName,
		configPath: p,
		gitClient:  client,
	}
	if m.gitClient != nil {
		var err error
		m.gitHeadRevision, err = m.gitClient.GetHeadRevision()
		if err != nil {
			logger.Error(err, "cannot get git head revision")
		}
	}

	if err := m.loadFromDisk(); err != nil {
		logger.Error(err, fmt.Sprintf("cannot load config file: %s", m.configPath))
		return nil, err
	}

	return &m, nil
}

// NewEmpty creates an empty Config Manager for Staging Controller
func NewEmpty() internal.ConfigManager {
	m := manager{}
	return &m
}

func addGitOptions(gitStorage s2hv1beta1.GitStorage, gitCred *s2hv1beta1.UsernamePasswordCredential) []s2hgit.Option {
	opts := make([]s2hgit.Option, 0)
	if gitCred != nil && gitCred.Password != "" {
		opts = append(opts, s2hgit.WithAuth(gitCred.Username, gitCred.Password))
	}

	if gitStorage.Ref != "" {
		opts = append(opts, s2hgit.WithReferenceName(gitStorage.Ref))
	}

	if gitStorage.CloneDepth > 0 {
		opts = append(opts, s2hgit.WithCloneDepth(gitStorage.CloneDepth))
	}

	if gitStorage.CloneTimeout != nil {
		opts = append(opts, s2hgit.WithCloneTimeout(gitStorage.CloneTimeout.Duration))
	}

	if gitStorage.PullTimeout != nil {
		opts = append(opts, s2hgit.WithPullTimeout(gitStorage.PullTimeout.Duration))
	}

	if gitStorage.PushTimeout != nil {
		opts = append(opts, s2hgit.WithPushTimeout(gitStorage.PushTimeout.Duration))
	}

	return opts
}

// load loads config from disk
func (m *manager) load(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if err := yaml.Unmarshal(data, &m.config); err != nil {
		return err
	}

	m.assignParent()

	if m.config.Envs == nil {
		m.config.Envs = map[string]internal.ComponentsValues{
			internal.EnvBase:      map[string]internal.ComponentValues{},
			internal.EnvStaging:   map[string]internal.ComponentValues{},
			internal.EnvPreActive: map[string]internal.ComponentValues{},
			internal.EnvActive:    map[string]internal.ComponentValues{},
		}
	}

	// load env specific values
	m.loadEnvsConfig(EnvsDir)

	return nil
}

func (m *manager) loadEnvsConfig(dir string) {
	listDirs := []string{internal.EnvBase, internal.EnvStaging, internal.EnvPreActive, internal.EnvActive}

	for _, l := range listDirs {
		comp := m.GetParentComponents()
		for i := range comp {
			v, err := m.loadEnvConfig(dir, l, comp[i].Name)
			if err != nil {
				continue
			}

			m.config.Envs[l][comp[i].Name] = v
		}
	}
}

func (m *manager) loadEnvConfig(dir, envName, componentName string) (internal.ComponentValues, error) {
	data, err := ioutil.ReadFile(path.Join(m.configPath, dir, envName, componentName+".yaml"))
	if err != nil {
		logger.Warn("cannot load config file",
			"env", envName, "component", componentName, "error", err.Error(), "team", m.teamName)
		return nil, err
	}

	var v internal.ComponentValues
	err = yaml.Unmarshal(data, &v)
	if err != nil {
		logger.Warn("cannot load unmarshal yaml",
			"env", envName, "component", componentName, "error", err.Error(), "team", m.teamName)
		return nil, err
	}

	return v, nil
}

func (m *manager) loadFromDisk() (err error) {
	var data []byte
	data, err = ioutil.ReadFile(path.Join(m.configPath, ConfigFileName))
	if err != nil {
		return
	}
	return m.load(data)
}

// assignParent assigns Parent to SubComponent
// only support 1 level of dependencies
func (m *manager) assignParent() {
	comps := m.config.Components

	for i := range m.config.Components {
		for j := range comps[i].Dependencies {
			comps[i].Dependencies[j].Parent = comps[i].Name
		}
	}
}

func (m *manager) Sync() error {
	// config path is not defined in case new with bytes
	if m.configPath == "" {
		return nil
	}

	teamName := m.teamName

	// git client was created
	if m.gitClient != nil {
		// if git folder is not found, returns error
		var err error
		gitDir := m.gitClient.GetDirectoryPath()
		if _, err = os.Stat(gitDir); os.IsNotExist(err) {
			return s2herrors.ErrGitDirectoryNotFound
		}

		if err = gitPull(m.gitClient, teamName); err != nil {
			if err == git.NoErrAlreadyUpToDate {
				return nil
			}

			if !s2herrors.IsErrGitPulling(err) {
				logger.Error(err, "cannot pull the repository", "team", teamName)
			}

			return err
		}

		m.gitHeadRevision, err = m.gitClient.GetHeadRevision()
		if err != nil {
			logger.Error(err, "cannot get git head revision")
		}
	}

	if err := m.loadFromDisk(); err != nil {
		logger.Error(err, fmt.Sprintf("cannot load config file: %s", m.configPath))
		return err
	}

	return nil
}

func (m *manager) Get() *internal.Configuration {
	return m.config
}
func (m *manager) Load(config *internal.Configuration, gitRev string) {
	m.config = config
	m.gitHeadRevision = gitRev
}

func (m *manager) GetComponents() (filteredComps map[string]*internal.Component) {
	filteredComps = map[string]*internal.Component{}

	var comps []*internal.Component
	var comp *internal.Component

	comps = append(comps, m.config.Components...)

	for len(comps) > 0 {
		comp, comps = comps[0], comps[1:]
		if len(comp.Dependencies) > 0 {
			// add to comps
			for _, dep := range comp.Dependencies {
				comps = append(comps, &internal.Component{
					Parent: comp.Name,
					Name:   dep.Name,
					Image:  dep.Image,
					Source: dep.Source,
				})
			}
		}

		// TODO: comment due to the parent component doesn't have source
		//if comp.Source == nil {
		//	// ignore if no source provided
		//	continue
		//}

		if _, exist := filteredComps[comp.Name]; exist {
			// duplication component name
			logger.Warn(fmt.Sprintf("duplicate component: %s detected", comp.Name))
			continue
		}

		filteredComps[comp.Name] = comp
	}

	return filteredComps
}

func (m *manager) GetParentComponents() (filteredComps map[string]*internal.Component) {
	filteredComps = m.GetComponents()

	for name, v := range filteredComps {
		if v.Parent != "" {
			delete(filteredComps, name)
		}
	}

	return
}

func (m *manager) GetGitLatestRevision() string {
	if m.gitHeadRevision == "" {
		return "<unknown>"
	}
	return m.gitHeadRevision
}

func (m *manager) GetGitInfo() internal.GitInfo {
	if m.gitClient == nil {
		return internal.GitInfo{
			HeadRevision: m.gitHeadRevision,
		}
	}

	rev, err := m.gitClient.GetHeadRevision()
	if err != nil {
		logger.Error(err, "cannot get git head revision")
		rev = "<unknown>"
	}

	return internal.GitInfo{
		Name:         m.gitClient.GetName(),
		FullName:     m.gitClient.GetBranchName(),
		BranchName:   m.gitClient.GetPath(),
		HeadRevision: rev,
	}
}

func (m *manager) GetGitBranchName() string {
	if m.gitClient == nil {
		return ""
	}
	return m.gitClient.GetBranchName()
}

func (m *manager) HasGitChanges(gitStorage s2hv1beta1.GitStorage) bool {
	if m.gitClient == nil {
		return false
	}

	newConfigPath := path.Join(m.gitClient.GetDirectoryPath(), gitStorage.Path)
	url, ref, depth := m.gitClient.GetGitParams()
	if url != gitStorage.URL || ref != gitStorage.Ref || depth != gitStorage.CloneDepth || m.configPath != newConfigPath {
		return true
	}

	return false
}

func (m *manager) GetGitConfigPath() string {
	return m.configPath
}

func (m *manager) Clean() error {
	if m.gitClient == nil {
		return nil
	}

	return m.gitClient.Clean()
}

func gitClone(gitClient *s2hgit.Client, teamName string) error {
	// start cloning the repository
	if _, exist := getMap(isCloning, teamName); !exist {
		setMap(isCloning, teamName, processWithError{true, nil})

		go func() {
			if err := gitClient.Clone(); err != nil {
				logger.Error(err, fmt.Sprintf("cannot clone repository of %s", teamName))
				setMap(isCloning, teamName, processWithError{false, err})
				return
			}

			setMap(isCloning, teamName, processWithError{false, nil})
		}()
	}

	// still cloning
	if res, exist := getMap(isCloning, teamName); exist {
		if res.err != nil {
			return res.err
		}

		if res.processing {
			return s2herrors.ErrGitCloning
		}

		deleteMap(isCloning, teamName)
	}

	return nil
}

func gitPull(gitClient *s2hgit.Client, teamName string) error {
	// start pulling the repository
	if _, exist := getMap(isPulling, teamName); !exist {
		setMap(isPulling, teamName, processWithError{true, nil})

		go func() {
			if err := gitClient.Pull(); err != nil {
				setMap(isPulling, teamName, processWithError{false, err})
				return
			}

			setMap(isPulling, teamName, processWithError{false, nil})
		}()
	}

	// still pulling
	if res, exist := getMap(isPulling, teamName); exist {
		if res.processing {
			return s2herrors.ErrGitPulling
		}

		deleteMap(isPulling, teamName)
		if res.err != nil {
			return res.err
		}
	}

	return nil
}

func getMap(m map[string]processWithError, teamName string) (res processWithError, exist bool) {
	teamLock := multiLock.Get(teamName)
	teamLock.Wait()
	defer teamLock.Done()

	res, exist = m[teamName]
	return
}

func setMap(m map[string]processWithError, teamName string, process processWithError) {
	teamLock := multiLock.Get(teamName)
	teamLock.Wait()
	defer teamLock.Done()

	m[teamName] = process
}

func deleteMap(m map[string]processWithError, teamName string) {
	teamLock := multiLock.Get(teamName)
	teamLock.Wait()
	defer teamLock.Done()

	delete(m, teamName)
}
