package helpers

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type SMTPConfig struct {
	Host      string
	Port      string
	Username  string
	Password  string
	FromEmail string
	FromName  string
	AppURL    string
}

func GetSMTPConfig() SMTPConfig {
	_ = godotenv.Overload()

	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" {
		port = "587"
	}
	username := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	password := strings.TrimSpace(os.Getenv("SMTP_PASSWORD"))
	password = strings.ReplaceAll(password, " ", "")
	fromEmail := strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL"))
	if fromEmail == "" {
		fromEmail = username
	}
	if fromEmail == "" {
		fromEmail = "noreply@accounts.sliit.lk"
	}
	fromName := strings.TrimSpace(os.Getenv("SMTP_FROM_NAME"))
	if fromName == "" {
		fromName = "Mozilla Campus Club of SLIIT"
	}
	appURL := strings.TrimSpace(os.Getenv("APP_URL"))
	if appURL == "" {
		appURL = "http://localhost:3000"
	}

	return SMTPConfig{
		Host:      host,
		Port:      port,
		Username:  username,
		Password:  password,
		FromEmail: fromEmail,
		FromName:  fromName,
		AppURL:    appURL,
	}
}

// SendEmail sends a multipart MIME email (plain text + HTML) via SMTP.
func SendEmail(to []string, subject, htmlBody, plainTextBody string) error {
	cfg := GetSMTPConfig()

	if cfg.Host == "" {
		log.Printf("[SMTP Warning] SMTP_HOST not configured. Email to %v skipped. Subject: %s\n", to, subject)
		return nil
	}

	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	boundary := fmt.Sprintf("----=_Boundary_SLIIT_%d", time.Now().UnixNano())

	// Build headers
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	fromHeader := fmt.Sprintf("%s <%s>", cfg.FromName, cfg.FromEmail)

	var msgBuilder strings.Builder
	msgBuilder.WriteString(fmt.Sprintf("From: %s\r\n", fromHeader))
	msgBuilder.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ", ")))
	msgBuilder.WriteString(fmt.Sprintf("Subject: %s\r\n", encodedSubject))
	msgBuilder.WriteString("MIME-Version: 1.0\r\n")
	msgBuilder.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	msgBuilder.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msgBuilder.WriteString("\r\n")

	// Plain text alternative
	if plainTextBody != "" {
		msgBuilder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msgBuilder.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
		msgBuilder.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		msgBuilder.WriteString(wrapBase64(base64.StdEncoding.EncodeToString([]byte(plainTextBody))))
		msgBuilder.WriteString("\r\n")
	}

	// HTML alternative
	if htmlBody != "" {
		msgBuilder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msgBuilder.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
		msgBuilder.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		msgBuilder.WriteString(wrapBase64(base64.StdEncoding.EncodeToString([]byte(htmlBody))))
		msgBuilder.WriteString("\r\n")
	}

	msgBuilder.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	rawMessage := []byte(msgBuilder.String())

	// Dispatch using direct TLS (Port 465) or STARTTLS (Port 587/others)
	if cfg.Port == "465" {
		return sendMailDirectTLS(cfg, addr, to, rawMessage)
	}
	return sendMailSTARTTLS(cfg, addr, to, rawMessage)
}

func sendMailDirectTLS(cfg SMTPConfig, addr string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: cfg.Host,
	}

	dialer := &net.Dialer{Timeout: 15 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("SMTP TLS dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("SMTP client initialization failed: %w", err)
	}
	defer client.Quit()

	if cfg.Username != "" && cfg.Password != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(cfg.FromEmail); err != nil {
		return fmt.Errorf("SMTP MAIL command failed: %w", err)
	}

	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("SMTP RCPT command failed for %s: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA command failed: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("SMTP write body failed: %w", err)
	}

	return w.Close()
}

func sendMailSTARTTLS(cfg SMTPConfig, addr string, to []string, msg []byte) error {
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("SMTP dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Quit()

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: cfg.Host,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("SMTP STARTTLS failed: %w", err)
		}
	}

	if cfg.Username != "" && cfg.Password != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(cfg.FromEmail); err != nil {
		return fmt.Errorf("SMTP MAIL command failed: %w", err)
	}

	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("SMTP RCPT command failed for %s: %w", recipient, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA command failed: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("SMTP write body failed: %w", err)
	}

	return w.Close()
}

func wrapBase64(s string) string {
	var result strings.Builder
	const lineLen = 76
	for len(s) > lineLen {
		result.WriteString(s[:lineLen])
		result.WriteString("\r\n")
		s = s[lineLen:]
	}
	result.WriteString(s)
	return result.String()
}


// GenerateOTP generates a cryptographically secure 6-digit numeric OTP code.
func GenerateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// SendOTPEmail sends a 6-digit verification code to the user's email address.
func SendOTPEmail(toEmail, toName, otpCode string) error {
	log.Printf("[OTP Verification] To: %s (%s) | Code: %s\n", toEmail, toName, otpCode)

	subject := fmt.Sprintf("Your Verification Code: %s - SLIIT Mozilla Accounts", otpCode)

	plainText := fmt.Sprintf(`Hello %s,

Thank you for registering with SLIIT Mozilla Accounts!

Your 6-digit verification code is:

%s

Enter this code on the verification screen to activate your account.
This code will expire in 10 minutes.

If you did not create this account, please ignore this email.

Best regards,
Mozilla Campus Club of SLIIT
`, toName, otpCode)

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Verification Code</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f8fafc; color: #1e293b; margin: 0; padding: 20px; }
    .container { max-width: 520px; margin: 0 auto; background: #ffffff; border-radius: 16px; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -1px rgba(0, 0, 0, 0.03); border: 1px solid #e2e8f0; }
    .header { background: linear-gradient(135deg, #1e293b 0%%, #0f172a 100%%); padding: 32px 24px; text-align: center; }
    .header h1 { color: #ffffff; margin: 0; font-size: 22px; font-weight: 700; letter-spacing: -0.5px; }
    .header p { color: #f47624; margin: 6px 0 0 0; font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 1.5px; }
    .content { padding: 36px 32px; text-align: center; }
    .greeting { font-size: 18px; font-weight: 600; color: #0f172a; margin-top: 0; margin-bottom: 12px; text-align: left; }
    .text { font-size: 15px; line-height: 1.6; color: #475569; margin-bottom: 24px; text-align: left; }
    .otp-box { background: #fff7ed; border: 2px dashed #f47624; border-radius: 12px; padding: 20px; margin: 24px 0; text-align: center; }
    .otp-label { font-size: 12px; text-transform: uppercase; letter-spacing: 1.2px; color: #9a3412; font-weight: 700; margin-bottom: 8px; }
    .otp-code { font-size: 40px; font-weight: 800; letter-spacing: 10px; color: #ea580c; font-family: 'Courier New', Courier, monospace; display: inline-block; padding: 0 4px; }
    .note { font-size: 13px; color: #64748b; margin-top: 24px; text-align: left; line-height: 1.6; border-top: 1px solid #f1f5f9; padding-top: 16px; }
    .footer { background-color: #f8fafc; padding: 20px 32px; text-align: center; font-size: 12px; color: #94a3b8; border-top: 1px solid #e2e8f0; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>SLIIT Mozilla Accounts</h1>
      <p>Mozilla Campus Club of SLIIT</p>
    </div>
    <div class="content">
      <h2 class="greeting">Hi %s,</h2>
      <p class="text">
        Use the 6-digit verification code below to complete your registration and activate your SLIIT Mozilla account:
      </p>
      <div class="otp-box">
        <div class="otp-label">Your Verification Code</div>
        <div class="otp-code">%s</div>
      </div>
      <p class="note">
        ⏱️ This code will expire in <strong>10 minutes</strong>.<br>
        🔒 If you did not create an account, please safely ignore this email.
      </p>
    </div>
    <div class="footer">
      &copy; %d Mozilla Campus Club of SLIIT. All rights reserved.
    </div>
  </div>
</body>
</html>`, toName, otpCode, time.Now().Year())

	return SendEmail([]string{toEmail}, subject, htmlBody, plainText)
}

// SendVerificationEmail dispatches an activation OTP to the user's registered email address.
func SendVerificationEmail(toEmail, toName, token string) error {
	return SendOTPEmail(toEmail, toName, token)
}
