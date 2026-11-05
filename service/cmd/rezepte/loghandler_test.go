package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func logOne(h slog.Handler, level slog.Level, msg string, attrs ...slog.Attr) {
	r := slog.NewRecord(time.Date(2026, 11, 5, 20, 9, 20, 0, time.Local), level, msg, 0)
	r.AddAttrs(attrs...)
	_ = h.Handle(context.Background(), r)
}

func TestTextHandler(t *testing.T) {
	var buf bytes.Buffer
	h := newTextHandler(&buf, slog.LevelInfo, false)

	logOne(h, slog.LevelInfo, "demo: seeded", slog.Int("recipes", 12), slog.String("note", "a b"))
	logOne(h.WithAttrs([]slog.Attr{slog.String("k", "v")}).WithGroup("g"), slog.LevelWarn, "push: failed", slog.Int("n", 1))
	logOne(h, slog.LevelError, "rezepte: stopped")

	want := "20:09:20 INF demo: seeded recipes=12 note=\"a b\"\n" +
		"20:09:20 WRN push: failed k=v g.n=1\n" +
		"20:09:20 ERR rezepte: stopped\n"
	if got := buf.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug enabled at level info")
	}
}

func TestTextHandlerColorsOnlyTheLevel(t *testing.T) {
	var buf bytes.Buffer
	h := newTextHandler(&buf, nil, true)
	logOne(h, slog.LevelWarn, "push: failed", slog.String("err", "boom"))

	want := "20:09:20 \x1b[33mWRN\x1b[0m push: failed err=boom\n"
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if strings.Count(buf.String(), "\x1b[") != 2 {
		t.Errorf("more than the level colored: %q", buf.String())
	}
}

func TestUseColor(t *testing.T) {
	for _, tc := range []struct {
		name string
		tty  bool
		env  map[string]string
		want bool
	}{
		{"terminal", true, nil, true},
		{"no terminal", false, nil, false},
		{"forced without terminal", false, map[string]string{"FORCE_COLOR": "1"}, true},
		{"turned off on a terminal", true, map[string]string{"NO_COLOR": "1"}, false},
		{"both set, off wins", true, map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := useColor(tc.tty, func(k string) string { return tc.env[k] }); got != tc.want {
				t.Errorf("useColor = %v, want %v", got, tc.want)
			}
		})
	}
}
