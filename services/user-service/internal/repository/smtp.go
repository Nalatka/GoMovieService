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
	body := fmt.Sprintf("To: %s\r\nSubject: Welcome to Movie Streaming Platform\r\n\r\nHello %s,\r\nWelcome to Movie Streaming Platform.\r\n", to, username)
	done := make(chan error, 1)
	go func() {
		done <- smtp.SendMail(addr, auth, m.from, []string{to}, []byte(body))
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
