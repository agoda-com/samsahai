package internal

// UpdatingSource represents source for checking desired version of components
type UpdatingSource string

const (
	PublicRegistry UpdatingSource = "publicRegistry"
	PublicHarbor   UpdatingSource = "publicHarbor"
)

type SubComponent struct {
	Name   string          `json:"name" yaml:"name"`
	Image  ComponentImage  `json:"image" yaml:"image"`
	Source *UpdatingSource `json:"source,omitempty" yaml:"source,omitempty"`
}

type Component struct {
	Name         string          `json:"name" yaml:"name"`
	Chart        ComponentChart  `json:"chart" yaml:"chart"`
	Image        ComponentImage  `json:"image" yaml:"image"`
	Values       interface{}     `json:"values,omitempty" yaml:"values,omitempty"`
	ValuesFiles  []string        `json:"valuesFiles,omitempty" yaml:"valuesFiles,omitempty"`
	Source       *UpdatingSource `json:"source,omitempty" yaml:"source,omitempty"`
	Dependencies []*SubComponent `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

type ComponentImage struct {
	Repository string `json:"repository" yaml:"repository"`
	Tag        string `json:"tag" yaml:"tag"`
	Pattern    string `json:"pattern" yaml:"pattern"`
}

// ComponentChart
type ComponentChart struct {
	Repository string `json:"repository" yaml:"repository"`
	Name       string `json:"name" yaml:"name"`
	Version    string `json:"version,omitempty" yaml:"version,omitempty"`
}
