package lark

import (
	"context"
	"log"

	golark "github.com/go-lark/lark"
	"github.com/gopaytech/istio-upgrade-worker/settings"
	"github.com/gopaytech/istio-upgrade-worker/types"
)

func NewNotificationLark(settings settings.Settings) Lark {
	return Lark{
		Settings: settings,
	}
}

type Lark struct {
	Settings settings.Settings
}

func (s Lark) Send(ctx context.Context, upgrade types.Notification) error {
	bot := golark.NewNotificationBot(s.Settings.NotificationLarkWebhook)
	_, err := bot.GetTenantAccessTokenInternal(true)
	if err != nil {
		log.Printf("failed to get tenant access token internal lark: %v\n", err.Error())
		return err
	}

	content := golark.NewPostBuilder().
		Title(upgrade.Title).
		TextTag(upgrade.Message, 1, true).
		Render()
	buffer := golark.NewMsgBuffer(golark.MsgPost).Post(content).Build()

	_, err = bot.PostMessage(buffer)
	if err != nil {
		log.Printf("failed to send lark notification: %v\n", err.Error())
		return err
	}

	return nil
}
