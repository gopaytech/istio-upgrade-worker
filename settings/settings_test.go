package settings

import (
	"os"
	"testing"
)

func TestSettings_DryRunDefault(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "test")
	os.Setenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH", "/tmp/test")
	os.Setenv("NOTIFICATION_SLACK_WEBHOOK", "https://hooks.slack.com/test")
	defer func() {
		os.Unsetenv("CLUSTER_NAME")
		os.Unsetenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH")
		os.Unsetenv("NOTIFICATION_SLACK_WEBHOOK")
	}()

	s, err := NewSettings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.DryRun != false {
		t.Errorf("expected DryRun default to be false, got %v", s.DryRun)
	}
}

func TestSettings_LogDefaults(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "test")
	os.Setenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH", "/tmp/test")
	os.Setenv("NOTIFICATION_SLACK_WEBHOOK", "https://hooks.slack.com/test")
	defer func() {
		os.Unsetenv("CLUSTER_NAME")
		os.Unsetenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH")
		os.Unsetenv("NOTIFICATION_SLACK_WEBHOOK")
	}()

	s, err := NewSettings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.LogLevel != "info" {
		t.Errorf("expected LogLevel default to be 'info', got %v", s.LogLevel)
	}
	if s.LogFormat != "json" {
		t.Errorf("expected LogFormat default to be 'json', got %v", s.LogFormat)
	}
}

func TestSettings_WebhookValidation_SlackMissing(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "test")
	os.Setenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH", "/tmp/test")
	os.Setenv("NOTIFICATION_MODE", "slack")
	os.Setenv("NOTIFICATION_SLACK_WEBHOOK", "")
	defer func() {
		os.Unsetenv("CLUSTER_NAME")
		os.Unsetenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH")
		os.Unsetenv("NOTIFICATION_MODE")
		os.Unsetenv("NOTIFICATION_SLACK_WEBHOOK")
	}()

	_, err := NewSettings()
	if err == nil {
		t.Error("expected error for missing slack webhook")
	}
	if err != ErrSlackWebhookRequired {
		t.Errorf("expected ErrSlackWebhookRequired, got %v", err)
	}
}

func TestSettings_WebhookValidation_LarkMissing(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "test")
	os.Setenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH", "/tmp/test")
	os.Setenv("NOTIFICATION_MODE", "lark")
	os.Setenv("NOTIFICATION_LARK_WEBHOOK", "")
	defer func() {
		os.Unsetenv("CLUSTER_NAME")
		os.Unsetenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH")
		os.Unsetenv("NOTIFICATION_MODE")
		os.Unsetenv("NOTIFICATION_LARK_WEBHOOK")
	}()

	_, err := NewSettings()
	if err == nil {
		t.Error("expected error for missing lark webhook")
	}
	if err != ErrLarkWebhookRequired {
		t.Errorf("expected ErrLarkWebhookRequired, got %v", err)
	}
}

func TestSettings_WebhookValidation_SlackProvided(t *testing.T) {
	os.Setenv("CLUSTER_NAME", "test")
	os.Setenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH", "/tmp/test")
	os.Setenv("NOTIFICATION_MODE", "slack")
	os.Setenv("NOTIFICATION_SLACK_WEBHOOK", "https://hooks.slack.com/test")
	defer func() {
		os.Unsetenv("CLUSTER_NAME")
		os.Unsetenv("DEPLOYMENT_FREEZE_CONFIG_FILE_PATH")
		os.Unsetenv("NOTIFICATION_MODE")
		os.Unsetenv("NOTIFICATION_SLACK_WEBHOOK")
	}()

	_, err := NewSettings()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
