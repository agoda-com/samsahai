package component

const (
	ErrMissingComponentArgs     = Error("cannot new component: component name and current version should not be empty")
	ErrWrongFormatComponentArgs = Error("cannot new component: outdated days should not be negative number")
)

type Error string

func (e Error) Error() string { return string(e) }
