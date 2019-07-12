package internal

type Plugin interface {
	DesiredComponentChecker

	// GetComponentName returns component name when incoming webhook matched with plugin name.
	// Useful for converting incoming component name to matched with the internal one.
	GetComponentName(name string) string
}
