package component

import (
	"sort"
	"time"
)

// Component defines properties of component
type Component struct {
	Name    string `required:"true" json:"name"`
	Version string `required:"true" json:"version"`
	Image   *Image `json:"image,omitempty"`
}

// emptyValueValidate validates empty values
func (c *Component) emptyValueValidate() bool {
	return c.Name != "" && c.Version != ""
}

// OutdatedComponent defines properties of outdated component
type OutdatedComponent struct {
	CurrentComponent *Component `required:"true" json:"current_component"`
	NewComponent     *Component `json:"new_component"`
	OutdatedDays     int        `json:"outdated_days"`
}

// Image defines properties of image
type Image struct {
	Repository string `yaml:"repository" json:"repository"`
	Tag        string `yaml:"tag" json:"tag"`
	Timestamp  int64  `yaml:"timestamp" json:"timestamp,omitempty"`
	Time       string `yaml:"time" json:"time,omitempty"`
}

// ValuesFile defines properties of values file
type ValuesFile struct {
	Image Image `yaml:"image"`
}

// Option provides option when creating component/outdated component
type Option struct {
	key   string
	value interface{}
}

const (
	optionImage      = "image"
	optionNewVersion = "new-version"
)

// NewOptionImage set the image for component
func NewOptionImage(image *Image) Option {
	return Option{key: optionImage, value: image}
}

type versionTimestamp struct {
	version   string
	timestamp int64
}

// NewOptionNewVersion set the new version for outdated component
func NewOptionNewVersion(newVersion string, timestamp int64) Option {
	return Option{
		key:   optionNewVersion,
		value: versionTimestamp{newVersion, timestamp},
	}
}

// NewComponent creates a new component
func NewComponent(name, version string, options ...Option) (*Component, error) {
	component := &Component{
		Name:    name,
		Version: version,
	}

	for _, opt := range options {
		switch opt.key {
		case optionImage:
			component.Image = opt.value.(*Image)
		}
	}

	if v := component.emptyValueValidate(); !v {
		return nil, ErrMissingComponentArgs
	}

	return component, nil
}

// NewOutdatedComponent creates a new outdated component
func NewOutdatedComponent(name, currentVersion string, options ...Option) (*OutdatedComponent, error) {
	currentComponent, err := NewComponent(name, currentVersion)
	if err != nil {
		return nil, err
	}

	var (
		newVersion   string
		newTimestamp int64
	)
	for _, opt := range options {
		switch opt.key {
		case optionNewVersion:
			newVersion = opt.value.(versionTimestamp).version
			newTimestamp = opt.value.(versionTimestamp).timestamp
		}
	}

	newComponent := &Component{}
	outdatedDays := 0
	if newVersion != "" {
		newComponent, err = NewComponent(name, newVersion)
		if err != nil {
			return nil, err
		}
		if newVersion != currentVersion {
			outdatedDays = getOutdatedDays(newTimestamp)
		}
	}

	outdatedComponent := &OutdatedComponent{
		CurrentComponent: currentComponent,
		NewComponent:     newComponent,
		OutdatedDays:     outdatedDays,
	}

	return outdatedComponent, nil
}

// SortComponentsByOutdatedDays sorts components by outdated days descending order
func SortComponentsByOutdatedDays(components []OutdatedComponent) {
	sort.Slice(components, func(i, j int) bool { return components[i].OutdatedDays > components[j].OutdatedDays })
}

// getOutdatedDays calculates outdated days
func getOutdatedDays(newVersionTimestamp int64) int {
	now := time.Now()
	newDate := time.Unix(newVersionTimestamp, 0)
	days := now.Sub(newDate).Hours() / 24
	return int(days) + 1
}
