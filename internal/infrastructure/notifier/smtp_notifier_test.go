package notifier

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSMTPNotifierRejectsHeaderInjection(t *testing.T) {
	notifier := NewSMTPNotifier(
		"smtp.example.com",
		587,
		"sender@example.com",
		"password",
		"sender@example.com",
		[]string{"receiver@example.com"},
	)

	err := notifier.Send(context.Background(), "hello\r\nBcc: attacker@example.com", "<p>body</p>")
	if err == nil || !strings.Contains(err.Error(), "newline") {
		t.Fatalf("expected header injection error, got %v", err)
	}
}

func TestNotifierEnabledStates(t *testing.T) {
	if !NewSMTPNotifier("smtp.example.com", 587, "user", "password", "from@example.com", []string{"to@example.com"}).Enabled() {
		t.Fatal("expected SMTP notifier to be enabled")
	}
	if NewDisabledNotifier().Enabled() {
		t.Fatal("expected disabled notifier to report disabled")
	}
}

func TestSMTPNotifierHonorsCancelledContext(t *testing.T) {
	notifier := NewSMTPNotifier(
		"203.0.113.1",
		587,
		"sender@example.com",
		"password",
		"sender@example.com",
		[]string{"receiver@example.com"},
	)
	notifier.Timeout = time.Minute

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := notifier.Send(ctx, "subject", "<p>body</p>")
	if err == nil {
		t.Fatal("expected cancelled context error")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("cancelled send took too long: %v", time.Since(start))
	}
}

func TestBuildMessageUsesCRLFAndEncodesSubject(t *testing.T) {
	message := buildMessage(
		"sender@example.com",
		[]string{"receiver@example.com"},
		"价格提醒",
		"<p>body</p>",
	)

	if !strings.Contains(message, "Subject: =?UTF-8?") {
		t.Fatalf("expected encoded UTF-8 subject, got %q", message)
	}
	if !strings.Contains(message, "\r\n\r\n<p>body</p>") {
		t.Fatalf("expected RFC-style header separator, got %q", message)
	}
}
