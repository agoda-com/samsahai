package send

const (
	ErrMissingSlackArguments = Error("slack-access-token/slack-channels argument is required")
	ErrEnvCannotBind         = Error("environment variable cannot be binded")
)

type Error string

func (e Error) Error() string { return string(e) }
