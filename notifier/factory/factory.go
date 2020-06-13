package factory

import (
	"strings"

	"github.com/nazo/lunchbox/config"
	"github.com/nazo/lunchbox/notifier"
	"github.com/nazo/lunchbox/notifier/slack"
)

func New(config *config.ConfigNotification) notifier.Notifier {
	switch driver := strings.ToLower(config.Driver); driver {
	case "slack":
		return slack.New(config.Slack)
	default:
		return nil
	}
}
