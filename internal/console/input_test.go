package console

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestInputKeepsLinesBetweenConsumers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	input := NewInput(ctx, strings.NewReader("2\nhello\nq\n"))
	for _, want := range []string{"2", "hello", "q"} {
		got, err := input.Read(ctx)
		if err != nil || got != want {
			t.Fatalf("got %q, %v; want %q", got, err, want)
		}
	}
	if _, err := input.Read(ctx); !errors.Is(err, io.EOF) {
		t.Fatalf("got %v; want EOF", err)
	}
}

func TestInputReadCanBeCancelledWhileTerminalIsIdle(t *testing.T) {
	appCtx, stop := context.WithCancel(context.Background())
	defer stop()
	source, keyboard := io.Pipe()
	defer source.Close()
	defer keyboard.Close()
	input := NewInput(appCtx, source)
	sessionCtx, cancel := context.WithCancel(appCtx)
	cancel()
	if _, err := input.Read(sessionCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v; want context.Canceled", err)
	}
	// Cancelling one consumer must not stop the shared console reader.
	go func() { _, _ = io.WriteString(keyboard, "next\n") }()
	got, err := input.Read(appCtx)
	if err != nil || got != "next" {
		t.Fatalf("got %q, %v; want next", got, err)
	}
}
