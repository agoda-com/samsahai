package send

import (
	"log"
	"strings"

	"github.com/agoda-com/samsahai/internal/samsahai/reporter"
	restClient "github.com/agoda-com/samsahai/internal/samsahai/reporter/rest"

	emailClient "github.com/agoda-com/samsahai/internal/samsahai/reporter/email"

	slackClient "github.com/agoda-com/samsahai/internal/samsahai/reporter/slack"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// DefaultSlackUsername is the default value for slack-username flag
	DefaultSlackUsername = ""
	// DefaultSlackEnabled is the default value for slack flag
	DefaultSlackEnabled = true
	// DefaultEmailEnabled is the default value for email flag
	DefaultEmailEnabled = false
	// DefaultEmailSubject is the default value for email-subject flag
	DefaultEmailSubject = ""
	// DefaultEmailFrom is the default value for email-from flag
	DefaultEmailFrom = "notification@samsahai.com"
	// DefaultRestEnabled is the default value for rest flag
	DefaultRestEnabled = false
)

// envMap maps flag names to env vars
var envMap = map[string]string{
	"slack-token":    "SLACK_TOKEN",
	"slack-channels": "SLACK_CHANNELS",
	"email-server":   "EMAIL_SERVER",
	"email-port":     "EMAIL_PORT",
	"email-from":     "EMAIL_FROM",
	"email-to":       "EMAIL_TO",
	"rest-endpoint":  "REST_ENDPOINT",
}

// slackArgs defines arguments of slack
type slackArgs struct {
	accessToken string
	channels    string
	username    string
	enabled     bool
}

// emailArgs defines arguments of email
type emailArgs struct {
	server  string
	port    int
	from    string
	to      string
	subject string
	enabled bool
}

// restArgs defines arguments of rest
type restArgs struct {
	endpoint string
	enabled  bool
}

var (
	slack slackArgs
	email emailArgs
	rest  restArgs
)

var reporters = make([]reporter.Reporter, 0)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "send",
		Short:             "send [active-promotion|outdated-component|image-missing|component-upgrade-fail]",
		PersistentPreRunE: preRun,
	}

	cmd.PersistentFlags().StringVar(&slack.accessToken, "slack-token", "", "Access token for slack app. Overrides $SLACK_TOKEN")
	cmd.PersistentFlags().StringVar(&slack.channels, "slack-channels", "", "Slack channels(slackClient) separate with comma(,). Overrides $SLACK_CHANNELS")
	cmd.PersistentFlags().StringVar(&slack.username, "slack-username", DefaultSlackUsername, "Override slack app username (depends on type of reporter)")
	cmd.PersistentFlags().BoolVar(&slack.enabled, "slack", DefaultSlackEnabled, "Flag enable/disable slack notification")

	cmd.PersistentFlags().StringVar(&email.server, "email-server", "", "SMTP server. Overrides $EMAIL_SERVER")
	cmd.PersistentFlags().IntVar(&email.port, "email-port", -1, "SMTP port. Overrides $EMAIL_PORT")
	cmd.PersistentFlags().StringVar(&email.from, "email-from", DefaultEmailFrom, "Source of email address(es) separate with comma(,). Overrides $EMAIL_FROM")
	cmd.PersistentFlags().StringVar(&email.to, "email-to", "", "Destination of email address(es) separate with comma(,). Overrides $EMAIL_TO")
	cmd.PersistentFlags().StringVar(&email.subject, "email-subject", DefaultEmailSubject, "Overrides email subject (depends on type of reporter)")
	cmd.PersistentFlags().BoolVar(&email.enabled, "email", DefaultEmailEnabled, "Flag enable/disable email notification")

	cmd.PersistentFlags().StringVar(&rest.endpoint, "rest-endpoint", "", "Endpoint for sending notification. Overrides $REST_ENDPOINT")
	cmd.PersistentFlags().BoolVar(&rest.enabled, "rest", DefaultRestEnabled, "Flag enable/disable sending http POST")

	bindPFlags(cmd)
	cmd.AddCommand(activePromotionCmd())
	cmd.AddCommand(imageMissingCmd())
	cmd.AddCommand(outdatedComponentsCmd())
	cmd.AddCommand(componentUpgradeFailCmd())
	return cmd
}

// preRun runs before command were executed
func preRun(cmd *cobra.Command, args []string) error {
	for _, env := range envMap {
		if err := viper.BindEnv(env); err != nil {
			return ErrBindingEnv
		}
	}

	slack.accessToken = viper.GetString(envMap["slack-token"])
	slack.channels = viper.GetString(envMap["slack-channels"])

	email.server = viper.GetString(envMap["email-server"])
	email.port = viper.GetInt(envMap["email-port"])
	email.from = viper.GetString(envMap["email-from"])
	email.to = viper.GetString(envMap["email-to"])

	rest.endpoint = viper.GetString(envMap["rest-endpoint"])

	if slack.enabled {
		slackCli, err := newSlackReporter()
		if err != nil {
			return err
		}
		reporters = append(reporters, slackCli)
	}

	if email.enabled {
		emailReporter, err := newEmailReporter()
		if err != nil {
			return err
		}
		reporters = append(reporters, emailReporter)
	}

	if rest.enabled {
		restReporter, err := newRestReporter()
		if err != nil {
			return err
		}
		reporters = append(reporters, restReporter)
	}

	return nil
}

// bindPFlags binds persistent flags from env map
func bindPFlags(cmd *cobra.Command) {
	for name, env := range envMap {
		if err := viper.BindPFlag(env, cmd.PersistentFlags().Lookup(name)); err != nil {
			log.Fatalf("%slackClient flag cannot be set", name)
		}
	}

}

func newSlackReporter() (*slackClient.Slack, error) {
	if validated, err := validateSlackArgs(&slack); !validated {
		return nil, err
	}

	channels := getMultipleValues(slack.channels)
	slackReporter := slackClient.NewSlack(slack.accessToken, slack.username, channels)
	return slackReporter, nil
}

func newEmailReporter() (*emailClient.Email, error) {
	if validated, err := validateEmailArgs(&email); !validated {
		return nil, err
	}

	froms := getMultipleValues(email.from)
	tos := getMultipleValues(email.to)
	emailReporter := emailClient.NewEmail(email.server, email.port, froms, tos)
	return emailReporter, nil
}

func newRestReporter() (*restClient.Rest, error) {
	if validated, err := validateRestArgs(&rest); !validated {
		return nil, err
	}

	restReporter := restClient.NewRest(rest.endpoint)
	return restReporter, nil
}

func validateSlackArgs(slackArgs *slackArgs) (bool, error) {
	if slackArgs.accessToken == "" {
		return false, ErrMissingSlackAccessTokenArgument
	}
	if slackArgs.channels == "" {
		return false, ErrMissingSlackChannelsArgument
	}

	return true, nil
}

func validateEmailArgs(emailArgs *emailArgs) (bool, error) {
	if emailArgs.server == "" {
		return false, ErrMissingEmailServerArgument
	}
	if emailArgs.port == -1 {
		return false, ErrMissingEmailPortArgument
	}
	if emailArgs.to == "" {
		return false, ErrMissingEmailToArgument
	}

	return true, nil
}

func validateRestArgs(restArgs *restArgs) (bool, error) {
	if restArgs.endpoint == "" {
		return false, ErrMissingRestEndpointArgument
	}

	return true, nil
}

func getMultipleValues(value string) []string {
	return strings.Split(value, ",")
}
