package repository

import (
	"context"
	"fmt"
	"net/smtp"
)

type SMTPMailer struct {
	host     string
	port     string
	username string
	password string
	from     string
}

func NewSMTPMailer(host string, port string, username string, password string, from string) *SMTPMailer {
	return &SMTPMailer{host: host, port: port, username: username, password: password, from: from}
}

func (m *SMTPMailer) Enabled() bool {
	return m.host != "" && m.port != "" && m.username != "" && m.password != "" && m.from != ""
}

func (m *SMTPMailer) SendWelcome(ctx context.Context, to string, username string) error {
	if !m.Enabled() {
		return nil
	}
	addr := m.host + ":" + m.port
	auth := smtp.PlainAuth("", m.username, m.password, m.host)
	msg := buildWelcomeEmail(m.from, to, username)
	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(addr, auth, m.from, []string{to}, msg)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func buildWelcomeEmail(from, to, username string) []byte {
	subject := "Welcome to Movie Streaming Platform"
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<body style="font-family:Arial,sans-serif;background:#f4f4f4;padding:20px">
  <div style="max-width:480px;margin:auto;background:#fff;border-radius:8px;padding:32px">
    <h2 style="color:#4f46e5">Welcome, %s!</h2>
    <p>Your account has been created on <strong>Movie Streaming Platform</strong>.</p>
    <p>You can now browse movies, build your watchlist and start streaming.</p>
    <p style="color:#888;font-size:12px">If you did not create this account, please ignore this email.</p>
  </div>
</body>
</html>`, username)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, to, subject, html,
	)
	return []byte(msg)
}
