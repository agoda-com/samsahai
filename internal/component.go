package internal

// UpdatingSource represents source for checking desired version of components
type UpdatingSource string

const (
	PublicRegistry UpdatingSource = "publicRegistry"
	PublicHarbor   UpdatingSource = "publicHarbor"
)

// Component represents a unit in Samsahai
type Component struct {
	Parent       string                 `json:"parent,omitempty"`
	Name         string                 `json:"name" yaml:"name"`
	Chart        ComponentChart         `json:"chart" yaml:"chart"`
	Image        ComponentImage         `json:"image" yaml:"image"`
	Values       map[string]interface{} `json:"values,omitempty" yaml:"values,omitempty"`
	Source       *UpdatingSource        `json:"source,omitempty" yaml:"source,omitempty"`
	Dependencies []*Component           `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

// ComponentImage
type ComponentImage struct {
	Repository string `json:"repository" yaml:"repository"`
	Tag        string `json:"tag" yaml:"tag"`
	Pattern    string `json:"pattern,omitempty" yaml:"pattern"`
}

// ComponentChart
type ComponentChart struct {
	Repository string `json:"repository" yaml:"repository"`
	Name       string `json:"name" yaml:"name"`
	Version    string `json:"version,omitempty" yaml:"version,omitempty"`
}

// ComponentValues
type ComponentValues map[string]interface{}

// ComponentsValues
type ComponentsValues map[string]ComponentValues
