package notification

import (
	"context"

	"github.com/gopaytech/istio-upgrade-worker/types"
)

type NotificationInterface interface {
	Send(ctx context.Context, upgrade types.Notification) error
}
