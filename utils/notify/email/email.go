package email

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	configure "github.com/WnJee/gorig/utils/cofigure"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	SSL      bool
	To       []string
}

func loadConfig() *Config {
	host := configure.GetString("notify.email.host")
	if host == "" {
		return nil
	}
	port := configure.GetInt("notify.email.port", 465)
	user := configure.GetString("notify.email.username")
	pass := configure.GetString("notify.email.password")
	from := configure.GetString("notify.email.from", user)
	ssl := configure.GetBool("notify.email.ssl", port == 465)
	to := configure.GetStringSlice("notify.email.to")

	return &Config{
		Host:     host,
		Port:     port,
		Username: user,
		Password: pass,
		From:     from,
		SSL:      ssl,
		To:       to,
	}
}

// Send sends an email to specified recipients with subject and body (text or HTML).
func Send(to []string, subject, body string, isHTML ...bool) error {
	cfg := loadConfig()
	if cfg == nil {
		return fmt.Errorf("email notify configuration not found (notify.email.host)")
	}

	if len(to) == 0 {
		to = cfg.To
	}
	if len(to) == 0 {
		return fmt.Errorf("email recipient list is empty")
	}

	html := false
	if len(isHTML) > 0 && isHTML[0] {
		html = true
	}

	contentType := "text/plain; charset=UTF-8"
	if html {
		contentType = "text/html; charset=UTF-8"
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	from := cfg.From
	if from == "" {
		from = cfg.Username
	}

	header := make(map[string]string)
	header["From"] = from
	header["To"] = strings.Join(to, ";")
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = contentType
	header["Date"] = time.Now().Format(time.RFC1123Z)

	var msgBuilder strings.Builder
	for k, v := range header {
		msgBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msgBuilder.WriteString("\r\n")
	msgBuilder.WriteString(body)

	message := []byte(msgBuilder.String())

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	if cfg.SSL || cfg.Port == 465 {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         cfg.Host,
		}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("dial smtp tls failed: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("create smtp client failed: %w", err)
		}
		defer client.Close()

		if auth != nil {
			if err = client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth failed: %w", err)
			}
		}

		if err = client.Mail(from); err != nil {
			return fmt.Errorf("smtp mail from failed: %w", err)
		}

		for _, rec := range to {
			if err = client.Rcpt(rec); err != nil {
				return fmt.Errorf("smtp rcpt to failed for %s: %w", rec, err)
			}
		}

		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp data writer failed: %w", err)
		}
		if _, err = w.Write(message); err != nil {
			return fmt.Errorf("smtp write body failed: %w", err)
		}
		if err = w.Close(); err != nil {
			return fmt.Errorf("smtp close data writer failed: %w", err)
		}
		return client.Quit()
	}

	// Standard STARTTLS or plain SMTP
	return smtp.SendMail(addr, auth, from, to, message)
}

// SendText sends a plain text email.
func SendText(to []string, subject, body string) error {
	return Send(to, subject, body, false)
}

// SendHTML sends an HTML formatted email.
func SendHTML(to []string, subject, htmlBody string) error {
	return Send(to, subject, htmlBody, true)
}
