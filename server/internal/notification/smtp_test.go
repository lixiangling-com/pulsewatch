package notification

import (
	"context"
	"testing"
)

func TestSMTPSenderRejectsHeaderInjectionWithoutDialing(t *testing.T) {
	sender := SMTPSender{Addr: "127.0.0.1:1025", From: "alerts@pulsewatch.local"}
	if err := sender.Send(context.Background(), "bad\r\nBcc: x@example.com", "subject", "body", ""); err == nil {
		t.Fatal("expected header injection rejection")
	}
}
