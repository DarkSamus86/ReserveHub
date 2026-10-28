package delivery

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/textproto"
	"net/url"
	"strings"
	"time"
)

type SMTP struct {
	Address, From, Username, Password string
	LinkBase                          *url.URL
}

func NewSMTP(address, from, username, password, link string) (*SMTP, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return nil, errors.New("invalid SMTP address")
	}
	parsedFrom, err := mail.ParseAddress(from)
	if err != nil || parsedFrom.Address != from || strings.ContainsAny(from, "\r\n") {
		return nil, errors.New("invalid SMTP sender")
	}
	if username == "" || password == "" {
		return nil, errors.New("SMTP credentials required")
	}
	base, err := url.Parse(link)
	if err != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.Fragment != "" {
		return nil, errors.New("invitation link must be an HTTPS URL")
	}
	return &SMTP{Address: address, From: from, Username: username, Password: password, LinkBase: base}, nil
}

func (s *SMTP) SendInvitation(ctx context.Context, email, token string) error {
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || strings.ContainsAny(email, "\r\n") {
		return errors.New("invalid invitation recipient")
	}
	host, _, _ := net.SplitHostPort(s.Address)
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", s.Address)
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	defer conn.Close()
	deadline := time.Now().Add(10 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	client := textproto.NewConn(conn)
	if _, _, err = client.ReadResponse(220); err != nil {
		return fmt.Errorf("SMTP greeting: %w", err)
	}
	if err = command(client, 250, "EHLO reservehub.local"); err != nil {
		return err
	}
	if err = command(client, 220, "STARTTLS"); err != nil {
		return fmt.Errorf("SMTP STARTTLS required: %w", err)
	}
	tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err = tlsConn.HandshakeContext(ctx); err != nil {
		return fmt.Errorf("secure SMTP connection: %w", err)
	}
	client = textproto.NewConn(tlsConn)
	if err = command(client, 250, "EHLO reservehub.local"); err != nil {
		return err
	}
	credential := base64.StdEncoding.EncodeToString([]byte("\x00" + s.Username + "\x00" + s.Password))
	if err = command(client, 235, "AUTH PLAIN "+credential); err != nil {
		return fmt.Errorf("authenticate SMTP: %w", err)
	}
	if err = command(client, 250, "MAIL FROM:<"+s.From+">"); err != nil {
		return err
	}
	if err = command(client, 250, "RCPT TO:<"+email+">"); err != nil {
		return err
	}
	if err = command(client, 354, "DATA"); err != nil {
		return err
	}
	w := client.DotWriter()
	link := *s.LinkBase
	q := link.Query()
	q.Set("token", token)
	link.RawQuery = q.Encode()
	message := "From: " + s.From + "\r\nTo: " + email + "\r\nSubject: ReserveHub invitation\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nOpen this invitation link before it expires:\r\n" + link.String() + "\r\n"
	if _, err = w.Write([]byte(message)); err != nil {
		_ = w.Close()
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	if _, _, err = client.ReadResponse(250); err != nil {
		return err
	}
	_ = command(client, 221, "QUIT")
	return nil
}

func command(client *textproto.Conn, expected int, line string) error {
	if err := client.PrintfLine("%s", line); err != nil {
		return err
	}
	if _, _, err := client.ReadResponse(expected); err != nil {
		return err
	}
	return nil
}
