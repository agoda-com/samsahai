package internal

type DesiredComponentChecker interface {

	// GetName returns name of checker
	GetName() string

	// GetVersion
	GetVersion(repository string, name string, pattern string) (string, error)

	// GetDailyVersion
	//GetDailyVersion(repository, name string) (string, error)
}

type DesiredComponentController interface {

	// Start
	Start()

	// Stop
	Stop()

	// LoadConfiguration loads config file from storage
	LoadConfiguration()

	// AddChecker
	AddChecker(name string, checker DesiredComponentChecker)

	// TryCheck
	TryCheck(names ...string)

	// GetComponents returns `Component` from `Configuration`
	GetComponents(config Configuration) map[string]*Component

	// Clear removes all DesiredComponents
	Clear()
}
