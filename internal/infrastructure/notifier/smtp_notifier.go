package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

const defaultSMTPTimeout = 15 * time.Second

// SMTPNotifier implements domain.INotifier using the standard library net/smtp
type SMTPNotifier struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
	Timeout  time.Duration
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
		Timeout:  defaultSMTPTimeout,
	}
}

// Send implements domain.INotifier
func (n *SMTPNotifier) Send(ctx context.Context, subject, body string) error {
	if err := validateHeaderValue("subject", subject); err != nil {
		return err
	}
	if err := validateHeaderValue("from", n.From); err != nil {
		return err
	}
	for _, recipient := range n.To {
		if err := validateHeaderValue("recipient", recipient); err != nil {
			return err
		}
	}
	if strings.TrimSpace(n.Host) == "" || n.Port <= 0 || n.Port > 65535 {
		return fmt.Errorf("invalid SMTP address")
	}
	if len(n.To) == 0 {
		return fmt.Errorf("no SMTP recipients configured")
	}

	timeout := n.Timeout
	if timeout <= 0 {
		timeout = defaultSMTPTimeout
	}
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	addr := net.JoinHostPort(n.Host, strconv.Itoa(n.Port))
	conn, err := dialSMTP(sendCtx, addr, n.Host, n.Port == 465)
	if err != nil {
		return fmt.Errorf("dial SMTP server: %w", err)
	}
	defer conn.Close()

	if deadline, ok := sendCtx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set SMTP deadline: %w", err)
		}
	}

	stopClose := make(chan struct{})
	go func() {
		select {
		case <-sendCtx.Done():
			_ = conn.Close()
		case <-stopClose:
		}
	}()
	defer close(stopClose)

	client, err := smtp.NewClient(conn, n.Host)
	if err != nil {
		return smtpContextError(sendCtx, "create SMTP client", err)
	}
	defer client.Close()

	if n.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(smtpTLSConfig(n.Host)); err != nil {
			return smtpContextError(sendCtx, "start SMTP TLS", err)
		}
	}

	if strings.TrimSpace(n.Username) != "" {
		auth := smtp.PlainAuth("", n.Username, n.Password, n.Host)
		if err := client.Auth(auth); err != nil {
			return smtpContextError(sendCtx, "authenticate SMTP", err)
		}
	}
	if err := client.Mail(n.From); err != nil {
		return smtpContextError(sendCtx, "set SMTP sender", err)
	}
	for _, recipient := range n.To {
		if err := client.Rcpt(recipient); err != nil {
			return smtpContextError(sendCtx, "set SMTP recipient", err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return smtpContextError(sendCtx, "open SMTP message", err)
	}
	if _, err := io.WriteString(writer, buildMessage(n.From, n.To, subject, body)); err != nil {
		_ = writer.Close()
		return smtpContextError(sendCtx, "write SMTP message", err)
	}
	if err := writer.Close(); err != nil {
		return smtpContextError(sendCtx, "close SMTP message", err)
	}
	if err := client.Quit(); err != nil {
		return smtpContextError(sendCtx, "quit SMTP session", err)
	}
	return nil
}

func dialSMTP(ctx context.Context, addr, host string, implicitTLS bool) (net.Conn, error) {
	dialer := &net.Dialer{}
	if implicitTLS {
		tlsDialer := &tls.Dialer{
			NetDialer: dialer,
			Config:    smtpTLSConfig(host),
		}
		return tlsDialer.DialContext(ctx, "tcp", addr)
	}
	return dialer.DialContext(ctx, "tcp", addr)
}

func smtpTLSConfig(host string) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: host,
	}
}

func buildMessage(from string, recipients []string, subject, body string) string {
	headers := []string{
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		fmt.Sprintf("From: C2C Monitor <%s>", from),
		fmt.Sprintf("To: %s", strings.Join(recipients, ", ")),
		fmt.Sprintf("Subject: %s", mime.QEncoding.Encode("UTF-8", subject)),
		"Date: " + time.Now().Format(time.RFC1123Z),
	}
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + body
}

func validateHeaderValue(name, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s contains invalid newline characters", name)
	}
	return nil
}

func smtpContextError(ctx context.Context, operation string, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return fmt.Errorf("%s: %w", operation, err)
}
