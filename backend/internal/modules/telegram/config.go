// Package telegram implements the Telegram bot (CR4): account linking by short rotating codes,
// mirroring of in-app notifications to the linked chat and a small command set that calls the same
// services as the web API. Nothing starts unless a bot token is configured.
package telegram

import (
	"net/url"
	"strings"
)

// Config is the module configuration. The token is a secret: it is never logged or serialized.
type Config struct {
	Token      string // TELEGRAM_BOT_TOKEN; empty disables the module
	BotURL     string // TELEGRAM_BOT_URL, https://t.me/<bot> shown by the UI / QR
	AppBaseURL string // APP_BASE_URL, deep links in messages
	APIBase    string // TELEGRAM_API_BASE, default https://api.telegram.org (tests point it to a fake)
}

// DefaultAPIBase is the public Telegram Bot API endpoint.
const DefaultAPIBase = "https://api.telegram.org"

// DefaultAppBaseURL is used when APP_BASE_URL is not set.
const DefaultAppBaseURL = "http://localhost:8080"

// Enabled reports whether a token is configured.
func (c Config) Enabled() bool { return strings.TrimSpace(c.Token) != "" }

// String never reveals the token.
func (c Config) String() string { return "telegram.Config{token: <redacted>}" }

// GoString never reveals the token.
func (c Config) GoString() string { return c.String() }

func (c Config) withDefaults() Config {
	c.Token = strings.TrimSpace(c.Token)
	c.BotURL = strings.TrimSpace(c.BotURL)
	c.AppBaseURL = strings.TrimRight(strings.TrimSpace(c.AppBaseURL), "/")
	if c.AppBaseURL == "" {
		c.AppBaseURL = DefaultAppBaseURL
	}
	c.APIBase = strings.TrimRight(strings.TrimSpace(c.APIBase), "/")
	if c.APIBase == "" {
		c.APIBase = DefaultAPIBase
	}
	return c
}

// buttonURLAllowed reports whether Telegram accepts u as an inline button URL. Telegram rejects
// localhost and private hosts, so for those the link is appended to the text instead.
func buttonURLAllowed(u string) bool {
	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "http" && p.Scheme != "https") {
		return false
	}
	h := strings.ToLower(p.Hostname())
	switch {
	case h == "" || h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".test"):
		return false
	case h == "::1" || strings.HasPrefix(h, "127.") || strings.HasPrefix(h, "10.") || strings.HasPrefix(h, "192.168.") || strings.HasPrefix(h, "169.254.") || strings.HasPrefix(h, "172."):
		return false
	}
	return true
}
