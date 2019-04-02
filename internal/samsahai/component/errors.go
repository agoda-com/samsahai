package component

const (
	// ErrMissingComponentArgs indicates that some component arguments are missing
	ErrMissingComponentArgs = Error("cannot new component: component name and current version should not be empty")
)

type Error string

// Error overrides error
func (e Error) Error() string { return string(e) }
