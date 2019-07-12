package internal

// DesiredComponentChecker represents standard interface for checking component version
type DesiredComponentChecker interface {
	// GetName returns name of checker
	GetName() string

	// GetVersion returns version from defined pattern
	GetVersion(repository string, name string, pattern string) (string, error)

	//EnsureVersion ensures the defined version is exist on repository
	EnsureVersion(repository string, name string, version string) error
}

type DesiredComponentController interface {
}
