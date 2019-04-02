package send

import (
	"log"
	"strings"

	s "github.com/agoda-com/samsahai/internal/samsahai/reporter/slack"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	// DefaultSlackUsername is the default value for slack-username flag
	DefaultSlackUsername = ""
	// DefaultSlackEnabled is the default value for slack flag
	DefaultSlackEnabled = true
)

// envMap maps flag names to env vars
var envMap = map[string]string{
	"slack-token":    "SLACK_TOKEN",
	"slack-channels": "SLACK_CHANNELS",
}

// slackArgs defines arguments of slack
type slackArgs struct {
	accessToken string
	channels    string
	username    string
	enabled     bool
}

var slack slackArgs

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "send",
		Short:             "send [active-promotion|outdated-component|image-missing|component-upgrade-fail]",
		PersistentPreRunE: preRun,
	}

	cmd.PersistentFlags().StringVar(&slack.accessToken, "slack-token", "", "Access token for slack app. Overrides $SLACK_TOKEN")
	cmd.PersistentFlags().StringVar(&slack.channels, "slack-channels", "", "Slack channels(s) separate with comma(,). Overrides $SLACK_CHANNELS")
	cmd.PersistentFlags().StringVar(&slack.username, "slack-username", DefaultSlackUsername, "Slack app username")
	cmd.PersistentFlags().BoolVar(&slack.enabled, "slack", DefaultSlackEnabled, "Flag enable/disable slack notification")

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
	return nil
}

// bindPFlags binds persistent flags from env map
func bindPFlags(cmd *cobra.Command) {
	for name, env := range envMap {
		if err := viper.BindPFlag(env, cmd.PersistentFlags().Lookup(name)); err != nil {
			log.Fatalf("%s flag cannot be set", name)
		}
	}

}

func newSlackReporter() (*s.Slack, error) {
	if validated, err := validateSlackArgs(&slack); !validated {
		return nil, err
	}

	channels := getSlackChannels(slack.channels)
	slackReporter := s.NewSlack(slack.accessToken, slack.username, channels)
	return slackReporter, nil
}

func validateSlackArgs(slackArgs *slackArgs) (bool, error) {
	if slackArgs.accessToken == "" || slackArgs.channels == "" {
		return false, ErrMissingSlackArguments
	}

	return true, nil
}

func getSlackChannels(channels string) []string {
	return getMultipleValues(channels)
}

func getMultipleValues(value string) []string {
	return strings.Split(value, ",")
}
