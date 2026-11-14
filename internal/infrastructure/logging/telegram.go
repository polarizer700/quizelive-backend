package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type TelegramConfig struct {
	Token   string
	ChatID  string
	Service string
}

type telegramSink struct {
	token   string
	chatID  string
	service string
	client  *http.Client

	mu       sync.Mutex
	lastSent map[string]time.Time
}

func newTelegramSink(cfg TelegramConfig) *telegramSink {
	return &telegramSink{
		token:    strings.TrimSpace(cfg.Token),
		chatID:   strings.TrimSpace(cfg.ChatID),
		service:  strings.TrimSpace(cfg.Service),
		client:   &http.Client{Timeout: 8 * time.Second},
		lastSent: map[string]time.Time{},
	}
}

func (s *telegramSink) enabled() bool {
	return s != nil && s.token != "" && s.chatID != ""
}

func (s *telegramSink) send(ctx context.Context, level slog.Level, msg string, attrs []slog.Attr) {
	if !s.enabled() {
		return
	}

	category := "general"
	var parts []string
	for _, attr := range attrs {
		if attr.Key == "category" {
			category = attr.Value.String()
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%v", attr.Key, attr.Value.Any()))
	}

	fingerprint := level.String() + "|" + category + "|" + msg
	s.mu.Lock()
	if last, ok := s.lastSent[fingerprint]; ok && time.Since(last) < 45*time.Second {
		s.mu.Unlock()
		return
	}
	s.lastSent[fingerprint] = time.Now()
	s.mu.Unlock()

	icon := "⚠️"
	if level >= slog.LevelError {
		icon = "🚨"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s <b>%s</b> · <code>%s</code>\n", icon, escapeHTML(level.String()), escapeHTML(s.service))
	fmt.Fprintf(&b, "<b>category</b>: <code>%s</code>\n", escapeHTML(category))
	fmt.Fprintf(&b, "<b>message</b>: %s\n", escapeHTML(msg))
	if len(parts) > 0 {
		fmt.Fprintf(&b, "<b>details</b>:\n<code>%s</code>", escapeHTML(strings.Join(parts, "\n")))
	}

	payload, _ := json.Marshal(map[string]any{
		"chat_id":                 s.chatID,
		"text":                    b.String(),
		"parse_mode":              "HTML",
		"disable_web_page_preview": true,
	})

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
}

func escapeHTML(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return replacer.Replace(value)
}

type telegramHandler struct {
	next slog.Handler
	sink *telegramSink
}

func (h *telegramHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *telegramHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level >= slog.LevelWarn && h.sink.enabled() {
		attrs := make([]slog.Attr, 0, record.NumAttrs())
		record.Attrs(func(attr slog.Attr) bool {
			attrs = append(attrs, attr)
			return true
		})
		go h.sink.send(context.WithoutCancel(ctx), record.Level, record.Message, attrs)
	}
	return h.next.Handle(ctx, record)
}

func (h *telegramHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &telegramHandler{next: h.next.WithAttrs(attrs), sink: h.sink}
}

func (h *telegramHandler) WithGroup(name string) slog.Handler {
	return &telegramHandler{next: h.next.WithGroup(name), sink: h.sink}
}

// Configure installs a slog default that mirrors Warn/Error into Telegram when configured.
func Configure(cfg TelegramConfig) {
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	sink := newTelegramSink(cfg)
	slog.SetDefault(slog.New(&telegramHandler{next: base, sink: sink}))
}
