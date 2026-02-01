package notifier

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPNotifier implements domain.INotifier using the standard library net/smtp
type SMTPNotifier struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
}

// NewSMTPNotifier creates a new SMTPNotifier
func NewSMTPNotifier(host string, port int, username, password, from string, to []string) *SMTPNotifier {
	return &SMTPNotifier{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
		To:       to,
	}
}

// Send implements domain.INotifier
func (n *SMTPNotifier) Send(ctx context.Context, subject, body string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	addr := fmt.Sprintf("%s:%d", n.Host, n.Port)
	auth := smtp.PlainAuth("", n.Username, n.Password, n.Host)

	// Construct email message
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	subjectHeader := fmt.Sprintf("Subject: %s", subject)
	fromHeader := fmt.Sprintf("From: C2C Monitor <%s>", n.From)
	toHeader := fmt.Sprintf("To: %s", strings.Join(n.To,","))

	msg := []byte(fmt.Sprintf("%s\r\n%s\r\n%s\r\n%s\r\n%s", fromHeader, toHeader, subjectHeader, mime, body))

	return smtp.SendMail(addr, auth, n.From, n.To, msg)
}
