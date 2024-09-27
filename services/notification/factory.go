package notification

import (
	"fmt"

	"github.com/gopaytech/istio-upgrade-worker/services/notification/slack"
	"github.com/gopaytech/istio-upgrade-worker/settings"
)

func NotificationFactory(settings settings.Settings) (NotificationInterface, error) {
	if settings.NotificationMode == "slack" {
		return slack.NewNotificationSlack(settings), nil
	}

	return nil, fmt.Errorf("notification is not supported")
}
