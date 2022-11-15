package notifications

import (
	"fmt"
	"strings"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/slack"
)

type SlackSettings struct {
	Name       string         `mapstructure:"name,omitempty"`
	Type       string         `mapstructure:"type,omitempty"`
	Enabled    bool           `mapstructure:"enabled"`
	Notifier   *notify.Notify `mapstructure:"-,omitempty" hide:"true"`
	Subject    string         `mapstructure:"subject,omitempty"`
	Message    string         `mapstructure:"message,omitempty"`
	OauthToken string         `mapstructure:"oauthToken,omitempty" redacted:"true"`
	Channels   []string       `mapstructure:"channels,omitempty"`
}

func GenerateSlackService(token string, channels []string) (*slack.Slack, error) {
	if token == "" {
		return nil, fmt.Errorf("Token is empty")
	}
	if len(channels) == 0 {
		return nil, fmt.Errorf("Channels are empty")
	}

	slackService := slack.New(token)
	c := strings.Join(channels, ",")
	slackService.AddReceivers(c)

	return slackService, nil
}
