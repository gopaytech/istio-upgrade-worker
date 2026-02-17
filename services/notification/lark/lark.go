package lark

import (
	"context"

	golark "github.com/go-lark/lark"
	"github.com/gopaytech/istio-upgrade-worker/pkg/logger"
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

	content := golark.NewPostBuilder().
		Title(upgrade.Title).
		TextTag(upgrade.Message, 1, true).
		Render()
	buffer := golark.NewMsgBuffer(golark.MsgPost).Post(content).Build()

	// Use channel to handle context cancellation
	type result struct {
		err error
	}
	done := make(chan result, 1)

	go func() {
		_, err := bot.PostNotificationV2(buffer)
		done <- result{err: err}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-done:
		if r.err != nil {
			logger.Log().Error().Err(r.err).Msg("failed to send lark notification")
			return r.err
		}
		return nil
	}
}
