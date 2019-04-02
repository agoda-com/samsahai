package send

const (
	ErrMissingSlackArguments = Error("slack-access-token/slack-channels argument is required")
	ErrBindingEnv            = Error("cannot bind environment variable")
	ErrWrongImageFormat      = Error("docker image was wrong format")
)

type Error string

func (e Error) Error() string { return string(e) }
