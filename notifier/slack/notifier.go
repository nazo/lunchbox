package slack

import (
	"github.com/nazo/lunchbox/config"
	"github.com/nazo/lunchbox/notifier"
	"github.com/slack-go/slack"
)

type SlackNotifier struct {
	notifier.Notifier
	config *config.ConfigNotificationSlack
}

func New(config *config.ConfigNotificationSlack) notifier.Notifier {
	return &SlackNotifier{
		config: config,
	}
}

func (s *SlackNotifier) Notify(logOutput *notifier.LogOutput) error {
	slackClient := slack.New(s.config.Token)
	_, _, err := slackClient.PostMessage(s.config.Channel, slack.MsgOptionText(logOutput.Status, false))

	return err
}
