package service

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestLogServiceDown(t *testing.T) {
	var buf bytes.Buffer

	svc := &MonitorService{
		downEventLogger: log.New(&buf, "", 0),
	}

	svc.logServiceDown("Gate", errors.New("timeout"))

	got := buf.String()
	if !strings.Contains(got, `SERVICE_DOWN service="Gate"`) {
		t.Fatalf("expected service name in log, got %q", got)
	}
	if !strings.Contains(got, `details="timeout"`) {
		t.Fatalf("expected error details in log, got %q", got)
	}
}
