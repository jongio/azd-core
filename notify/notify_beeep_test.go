package notify

import (
	"context"
	"testing"
	"time"
)

func TestBeeepNotifier_New(t *testing.T) {
	config := DefaultConfig()
	notifier, err := newPlatformNotifier(config)
	if err != nil {
		t.Fatalf("failed to create beeep notifier: %v", err)
	}

	if notifier == nil {
		t.Fatal("expected notifier, got nil")
	}

	bn, ok := notifier.(*beeepNotifier)
	if !ok {
		t.Fatal("expected beeepNotifier type")
	}

	if bn.config.AppName != config.AppName {
		t.Errorf("expected app name %s, got %s", config.AppName, bn.config.AppName)
	}
}

func TestBeeepNotifier_IsAvailable(t *testing.T) {
	config := DefaultConfig()
	notifier, _ := newPlatformNotifier(config)

	if !notifier.IsAvailable() {
		t.Error("expected IsAvailable to return true")
	}
}

func TestBeeepNotifier_RequestPermission(t *testing.T) {
	config := DefaultConfig()
	notifier, _ := newPlatformNotifier(config)

	ctx := context.Background()
	err := notifier.RequestPermission(ctx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestBeeepNotifier_Close(t *testing.T) {
	config := DefaultConfig()
	notifier, _ := newPlatformNotifier(config)

	err := notifier.Close()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestBeeepNotifier_Config(t *testing.T) {
	config := Config{
		AppName: "Custom App",
		AppID:   "com.custom.app",
		Timeout: 10 * time.Second,
	}
	notifier, _ := newPlatformNotifier(config)
	bn := notifier.(*beeepNotifier)

	if bn.config.AppName != "Custom App" {
		t.Errorf("expected app name 'Custom App', got %s", bn.config.AppName)
	}
	if bn.config.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", bn.config.Timeout)
	}
}
