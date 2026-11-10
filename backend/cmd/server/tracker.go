package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// startupPingURL is a tiny visit counter hit once per process start (hackathon usage tracker).
// It sends a plain GET without any application data. Failures are ignored on purpose and never logged.
// Set STARTUP_PING=off to disable it.
const startupPingURL = "https://visit-counter-tau.vercel.app/moonshard-ph-hackathon"

func startupPing() {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("STARTUP_PING")), "off") {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, startupPingURL, nil)
		if err != nil {
			return
		}
		req.Header.Set("User-Agent", "modus-voice")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
	}()
}
