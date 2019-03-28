package component

type (
	Component struct {
		Name           string
		CurrentVersion string
		NewVersion     string
		OutdatedDays   int
	}

	ValuesFile struct {
		Image Image `yaml:"image"`
	}

	Image struct {
		Repository string `yaml:"repository"`
		Tag        string `yaml:"tag"`
		Timestamp  int64  `yaml:"timestamp"`
		Time       string `yaml:"time"`
	}
)

func NewComponent(name, currentVersion, newVersion string, outdatedDays int) (*Component, error) {
	component := &Component{
		Name:           name,
		CurrentVersion: currentVersion,
		NewVersion:     newVersion,
		OutdatedDays:   outdatedDays,
	}

	if v := component.emptyValueValidate(); !v {
		return nil, ErrMissingComponentArgs
	}
	if v := component.formatValidate(); !v {
		return nil, ErrWrongFormatComponentArgs
	}

	return component, nil
}

func (c *Component) emptyValueValidate() bool {
	return c.Name != "" && c.CurrentVersion != ""
}

func (c *Component) formatValidate() bool {
	return c.OutdatedDays >= 0
}
