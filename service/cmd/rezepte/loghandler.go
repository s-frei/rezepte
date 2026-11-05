package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
)

// textHandler writes "15:04:05 INF msg key=value": time and level short and
// first, the level colored when color is on. The attributes go through an
// inner slog.TextHandler, so quoting, groups and With stay the standard
// library's; only time, level and message are written here.
type textHandler struct {
	inner slog.Handler
	color bool
	out   *output
}

// output is shared by a handler and everything derived from it with
// WithAttrs or WithGroup: the inner handler renders into buf, then the whole
// line goes to w, under one lock so lines never interleave.
type output struct {
	mu  sync.Mutex
	w   io.Writer
	buf bytes.Buffer
}

func newTextHandler(w io.Writer, level slog.Leveler, color bool) slog.Handler {
	h := &textHandler{color: color, out: &output{w: w}}
	h.inner = slog.NewTextHandler(&h.out.buf, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && (a.Key == slog.TimeKey || a.Key == slog.LevelKey || a.Key == slog.MessageKey) {
				return slog.Attr{}
			}
			return a
		},
	})
	return h
}

// useColor turns color on for a terminal; FORCE_COLOR turns it on anywhere,
// NO_COLOR off everywhere, and NO_COLOR wins (https://no-color.org).
func useColor(terminal bool, getenv func(string) string) bool {
	if getenv("NO_COLOR") != "" {
		return false
	}
	return terminal || getenv("FORCE_COLOR") != ""
}

func (h *textHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	c := *h
	c.inner = h.inner.WithAttrs(attrs)
	return &c
}

func (h *textHandler) WithGroup(name string) slog.Handler {
	c := *h
	c.inner = h.inner.WithGroup(name)
	return &c
}

func (h *textHandler) Handle(ctx context.Context, r slog.Record) error {
	o := h.out
	o.mu.Lock()
	defer o.mu.Unlock()
	o.buf.Reset()
	if err := h.inner.Handle(ctx, r); err != nil {
		return err
	}
	attrs := o.buf.Bytes() // "k=v k=v\n", or just "\n"

	line := make([]byte, 0, 32+len(r.Message)+len(attrs))
	line = r.Time.AppendFormat(line, "15:04:05")
	line = append(line, ' ')
	name, code := levelLabel(r.Level)
	if h.color {
		line = append(line, "\x1b["+code+"m"+name+"\x1b[0m"...)
	} else {
		line = append(line, name...)
	}
	line = append(line, ' ')
	line = append(line, r.Message...)
	if len(attrs) > 1 {
		line = append(line, ' ')
	}
	line = append(line, attrs...)
	_, err := o.w.Write(line)
	return err
}

// levelLabel is the three-letter name and the ANSI color of a level: gray,
// blue, yellow, red.
func levelLabel(l slog.Level) (name, code string) {
	switch {
	case l >= slog.LevelError:
		return "ERR", "31"
	case l >= slog.LevelWarn:
		return "WRN", "33"
	case l >= slog.LevelInfo:
		return "INF", "34"
	default:
		return "DBG", "90"
	}
}
