package internal

import "time"

type Configuration struct {

	// TearDownDuration defines duration before tear down the old active
	TearDownDuration time.Duration `json:"tearDownDuration" yaml:"tearDownDuration"`

	// Components defines all components that are managed
	Components []*Component `json:"components" yaml:"components"`
}

type ConfigManager interface {
	// Sync keeps config synchronized with storage layer
	Sync() error

	// Get returns configuration
	Get() *Configuration
}
