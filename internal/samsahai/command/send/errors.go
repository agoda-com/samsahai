package send

const (
	ErrMissingSlackAccessTokenArgument = Error("slack-access-token argument is required")
	ErrMissingSlackChannelsArgument    = Error("slack-channels argument is required")
	ErrMissingEmailServerArgument      = Error("email-server argument is required")
	ErrMissingEmailPortArgument        = Error("email-port argument is required")
	ErrMissingEmailToArgument          = Error("email-to argument is required")
	ErrMissingRestEndpointArgument     = Error("rest-endpoint argument is required")
	ErrBindingEnv                      = Error("cannot bind environment variable")
	ErrWrongImageFormat                = Error("docker image was wrong format")
)

type Error string

func (e Error) Error() string { return string(e) }
