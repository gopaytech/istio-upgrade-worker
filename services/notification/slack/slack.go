package slack

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/gopaytech/istio-upgrade-worker/settings"
	"github.com/gopaytech/istio-upgrade-worker/types"
	"github.com/slack-go/slack"
)

func NewNotificationSlack(settings settings.Settings) Slack {
	return Slack{
		Settings: settings,
	}
}

type Slack struct {
	Settings settings.Settings
}

func (s Slack) Send(ctx context.Context, upgrade types.Notification) error {
	attachment := slack.Attachment{
		AuthorName: "istio-upgrade-worker",
		Title:      upgrade.Title,
		Text:       upgrade.Message,
		Ts:         json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
	}

	msg := slack.WebhookMessage{
		Attachments: []slack.Attachment{attachment},
	}

	err := slack.PostWebhook(s.Settings.NotificationSlackWebhook, &msg)
	if err != nil {
		log.Printf("failed to send slack notification: %v\n", err.Error())
		return err
	}

	return nil
}
