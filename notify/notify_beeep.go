package notify

import (
	"context"

	"github.com/gen2brain/beeep"
)

// beeepNotifier implements Notifier using the cross-platform beeep library.
type beeepNotifier struct {
	config Config
}

// newPlatformNotifier creates a beeep-based notifier.
func newPlatformNotifier(config Config) (Notifier, error) {
	return &beeepNotifier{
		config: config,
	}, nil
}

// Send sends a notification using beeep.
func (n *beeepNotifier) Send(_ context.Context, notification Notification) error {
	return beeep.Notify(notification.Title, notification.Message, "")
}

// IsAvailable returns true since beeep handles platform detection internally.
func (n *beeepNotifier) IsAvailable() bool {
	return true
}

// RequestPermission is a no-op since beeep handles permissions internally.
func (n *beeepNotifier) RequestPermission(_ context.Context) error {
	return nil
}

// Close is a no-op for beeep.
func (n *beeepNotifier) Close() error {
	return nil
}
