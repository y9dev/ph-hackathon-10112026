package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Update is the subset of a Bot API update the bot understands.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

// Message is an incoming message.
type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
	From *User  `json:"from"`
}

// Chat identifies the conversation.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// User is a Telegram account.
type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

// Button is an inline keyboard URL button.
type Button struct {
	Text string
	URL  string
}

// APIError is an error response of the Bot API.
type APIError struct {
	Code        int
	Description string
	RetryAfter  int // seconds, set for 429
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram api error %d: %s", e.Code, e.Description)
}

// API is what the bot and the mirror need from Telegram; *Client implements it.
type API interface {
	GetMe(ctx context.Context) (User, error)
	GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error)
	SendMessage(ctx context.Context, chatID int64, text string, btn *Button) error
	// SendLinked sends plain text where the trailing occurrence of link.Text is a hyperlink to link.URL.
	SendLinked(ctx context.Context, chatID int64, text string, link TextLink) error
}

const (
	// GlobalSendInterval keeps the whole bot under 25 messages per second.
	GlobalSendInterval = time.Second / 25
	// ChatSendInterval keeps a single chat at one message per second.
	ChatSendInterval = time.Second
	// PollTimeoutSeconds is the long polling timeout of getUpdates.
	PollTimeoutSeconds = 25
	maxRetryAfter      = 30
	maxSendTries       = 4
)

// Client is a minimal Bot API client on net/http. The token is part of request URLs only and is
// stripped from every returned error.
type Client struct {
	base  string
	token string
	hc    *http.Client
	lim   *sendLimiter
	sleep func(ctx context.Context, d time.Duration) error
}

// NewClient builds a client for cfg (defaults applied by the caller).
func NewClient(cfg Config) *Client {
	return &Client{
		base: cfg.APIBase, token: cfg.Token,
		hc:    &http.Client{Timeout: (PollTimeoutSeconds + 15) * time.Second},
		lim:   newSendLimiter(GlobalSendInterval, ChatSendInterval, time.Now),
		sleep: sleepCtx,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

// call performs one Bot API request. Errors never contain the token (net/http embeds the URL).
func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram %s: encode: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/bot"+c.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram %s: build request", method)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("telegram %s: %w", method, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("telegram %s: read body: %w", method, err)
	}
	var ar apiResponse
	if err := json.Unmarshal(raw, &ar); err != nil {
		return fmt.Errorf("telegram %s: http %d: undecodable response", method, resp.StatusCode)
	}
	if !ar.OK {
		code := ar.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		return &APIError{Code: code, Description: ar.Description, RetryAfter: ar.Parameters.RetryAfter}
	}
	if out != nil {
		if err := json.Unmarshal(ar.Result, out); err != nil {
			return fmt.Errorf("telegram %s: decode result: %w", method, err)
		}
	}
	return nil
}

// GetMe validates the token and returns the bot account.
func (c *Client) GetMe(ctx context.Context) (User, error) {
	var u User
	err := c.call(ctx, "getMe", struct{}{}, &u)
	return u, err
}

// GetUpdates long-polls for updates with id >= offset.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	var ups []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset": offset, "timeout": timeoutSec, "allowed_updates": []string{"message"},
	}, &ups)
	return ups, err
}

// TextLink turns the last occurrence of Text inside a message into a hyperlink to URL. It uses message
// entities instead of parse_mode, so the message text never needs escaping.
type TextLink struct {
	Text string
	URL  string
}

// SendMessage sends plain text (no parse_mode) honouring the rate limits and retrying on 429.
func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, btn *Button) error {
	return c.send(ctx, chatID, text, btn, nil)
}

// SendLinked sends plain text with one hyperlink (see TextLink). If Text is not found, no link is added.
func (c *Client) SendLinked(ctx context.Context, chatID int64, text string, link TextLink) error {
	var ents []map[string]any
	if i := strings.LastIndex(text, link.Text); i >= 0 && link.Text != "" {
		ents = append(ents, map[string]any{
			"type": "text_link", "url": link.URL,
			"offset": utf16Len(text[:i]), "length": utf16Len(link.Text),
		})
	}
	return c.send(ctx, chatID, text, nil, ents)
}

// utf16Len counts UTF-16 code units, the unit Telegram uses for entity offsets.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func (c *Client) send(ctx context.Context, chatID int64, text string, btn *Button, ents []map[string]any) error {
	payload := map[string]any{"chat_id": chatID, "text": text, "disable_web_page_preview": true}
	if len(ents) > 0 {
		payload["entities"] = ents
	}
	if btn != nil {
		payload["reply_markup"] = map[string]any{"inline_keyboard": [][]map[string]string{{{"text": btn.Text, "url": btn.URL}}}}
	}
	var err error
	for try := 0; try < maxSendTries; try++ {
		if werr := c.sleep(ctx, c.lim.reserveChat(chatID)); werr != nil {
			return werr
		}
		if werr := c.sleep(ctx, c.lim.reserveGlobal()); werr != nil {
			return werr
		}
		err = c.call(ctx, "sendMessage", payload, nil)
		var ae *APIError
		if err == nil || !errors.As(err, &ae) || ae.Code != http.StatusTooManyRequests {
			return err
		}
		wait := ae.RetryAfter
		if wait < 1 {
			wait = 1
		}
		if wait > maxRetryAfter {
			wait = maxRetryAfter
		}
		if serr := c.sleep(ctx, time.Duration(wait)*time.Second); serr != nil {
			return serr
		}
	}
	return err
}

// sendLimiter spaces outgoing messages: one per chat interval per chat and at most one per global
// interval overall. A send first waits for its chat slot (reserveChat) and only then books a
// global slot (reserveGlobal), so a chat with a backlog never blocks other chats.
type sendLimiter struct {
	mu         sync.Mutex
	global     time.Duration
	perChat    time.Duration
	now        func() time.Time
	nextGlobal time.Time
	nextChat   map[int64]time.Time
}

func newSendLimiter(global, perChat time.Duration, now func() time.Time) *sendLimiter {
	return &sendLimiter{global: global, perChat: perChat, now: now, nextChat: map[int64]time.Time{}}
}

func (l *sendLimiter) reserveChat(chatID int64) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	slot := now
	if t := l.nextChat[chatID]; t.After(slot) {
		slot = t
	}
	l.nextChat[chatID] = slot.Add(l.perChat)
	if len(l.nextChat) > 1024 { // drop chats whose slot has passed
		for id, t := range l.nextChat {
			if t.Before(now) {
				delete(l.nextChat, id)
			}
		}
	}
	return slot.Sub(now)
}

func (l *sendLimiter) reserveGlobal() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	slot := now
	if l.nextGlobal.After(slot) {
		slot = l.nextGlobal
	}
	l.nextGlobal = slot.Add(l.global)
	return slot.Sub(now)
}
