package send

import (
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Maps flag names to env vars
var envMap = map[string]string{
	"slack-token":    "SLACK_TOKEN",
	"slack-channels": "SLACK_CHANNELS",
}

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
		Short:             "send [active-promotion|outdated-component|image-missing]",
		PersistentPreRunE: preRun,
	}

	cmd.PersistentFlags().StringVar(&slack.accessToken, "slack-token", "", "Access token for slack app. Overrides $SLACK_TOKEN")
	cmd.PersistentFlags().StringVar(&slack.channels, "slack-channels", "", "Slack channels(s) separate with comma(,). Overrides $SLACK_CHANNELS")
	cmd.PersistentFlags().StringVar(&slack.username, "slack-username", "", "Slack app username")
	cmd.PersistentFlags().BoolVar(&slack.enabled, "slack", true, "Flag enable/disable slack notification")

	bindPFlags(cmd)
	cmd.AddCommand(activePromotionCmd())
	return cmd
}

// preRun runs before command were executed
func preRun(cmd *cobra.Command, args []string) error {
	for _, env := range envMap {
		if err := viper.BindEnv(env); err != nil {
			return ErrEnvCannotBind
		}
	}

	slack.accessToken = viper.GetString(envMap["slack-token"])
	slack.channels = viper.GetString(envMap["slack-channels"])
	return nil
}

func bindPFlags(cmd *cobra.Command) {
	for name, env := range envMap {
		if err := viper.BindPFlag(env, cmd.PersistentFlags().Lookup(name)); err != nil {
			log.Fatalf("%s flag cannot be set", name)
		}
	}

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
