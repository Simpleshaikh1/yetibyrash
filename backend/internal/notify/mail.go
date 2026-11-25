// Package notify sends customers their order messages (email now; WhatsApp/SMS in M6).
package notify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Mailer sends one plain-text email.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// SMTPMailer sends through an SMTP server: Mailpit (localhost:1025) in development,
// a provider such as Resend or Postmark in production.
type SMTPMailer struct {
	Addr     string // host:port
	From     string // "Yetibyrash <orders@example.com>"
	Username string // empty = no login (Mailpit)
	Password string
}

func (m SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	fromAddr := m.From
	if i := strings.LastIndex(fromAddr, "<"); i >= 0 {
		fromAddr = strings.Trim(fromAddr[i:], "<>")
	}
	host, _, _ := net.SplitHostPort(m.Addr)

	var msg strings.Builder
	fmt.Fprintf(&msg, "From: %s\r\n", m.From)
	fmt.Fprintf(&msg, "To: %s\r\n", to)
	fmt.Fprintf(&msg, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&msg, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&msg, "Message-ID: <%s@%s>\r\n", randomID(), domainOf(fromAddr))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	msg.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))

	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, host)
	}
	return smtp.SendMail(m.Addr, auth, fromAddr, []string{to}, []byte(msg.String()))
}

func randomID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func domainOf(addr string) string {
	if i := strings.LastIndex(addr, "@"); i >= 0 {
		return addr[i+1:]
	}
	return "localhost"
}
