package emailer

import (
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"

	emailverifier "github.com/AfterShip/email-verifier"
)

type Address string

func ParseAddress(raw string) (Address, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := mail.ParseAddress(raw)
	if err != nil || parsed.Address != raw {
		return "", fmt.Errorf("invalid email address")
	}
	syntax := emailverifier.NewVerifier().ParseAddress(raw)
	if !syntax.Valid {
		return "", fmt.Errorf("invalid email address")
	}
	return Address(strings.ToLower(raw)), nil
}

type Mailer struct {
	host, port, username, password, from string
}

func NewFromEnv() (*Mailer, error) {
	host, port, from := strings.TrimSpace(os.Getenv("SMTP_HOST")),
		strings.TrimSpace(os.Getenv("SMTP_PORT")), strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if host == "" || port == "" || from == "" {
		return nil, fmt.Errorf("SMTP_HOST, SMTP_PORT, and SMTP_FROM must be configured")
	}
	if _, err := ParseAddress(from); err != nil {
		return nil, fmt.Errorf("invalid SMTP_FROM: %w", err)
	}
	return &Mailer{host: host, port: port, username: os.Getenv("SMTP_USERNAME"),
		password: os.Getenv("SMTP_PASSWORD"), from: from}, nil
}

func (m *Mailer) Send(to Address, subject, body string) error {
	if m == nil {
		return fmt.Errorf("email delivery is not configured")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("invalid email subject")
	}
	hostPort := net.JoinHostPort(m.host, m.port)
	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}
	message := []byte("From: " + m.from + "\r\n" +
		"To: " + string(to) + "\r\n" + "Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	return smtp.SendMail(hostPort, auth, m.from, []string{string(to)}, message)
}

func (m *Mailer) SendSetupCode(to Address, code string, expiresMinutes int) error {
	body := fmt.Sprintf("Your FATV setup code is: %s\nIt expires in %d minutes.", code, expiresMinutes)
	if link := frontendURL("/setup-account"); link != "" { body += "\nSet your password: " + link }
	body += "\nIgnore this message if unexpected."
	return m.Send(to, "FATV account setup", body)
}

func (m *Mailer) SendRecoveryCode(to Address, code string, expiresMinutes int) error {
	body := fmt.Sprintf("Your FATV recovery code is: %s\nIt expires in %d minutes.", code, expiresMinutes)
	if link := frontendURL("/recover-password"); link != "" { body += "\nReset your password: " + link }
	body += "\nIgnore this message if unexpected."
	return m.Send(to, "FATV password recovery", body)
}

func frontendURL(path string) string {
	return strings.TrimRight(os.Getenv("FRONTEND_BASE_URL"), "/") + path
}

func (m *Mailer) SendFormInvitation(to Address, link, expiresAt string) error {
	return m.Send(to, "FATV form invitation",
		fmt.Sprintf("You have been invited to complete a FATV form.\nOpen this link: %s\nThis invitation expires at %s.", link, expiresAt))
}
