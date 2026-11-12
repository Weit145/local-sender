package connect

import (
	"bufio"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/Weit145/local-sender/internal/console"
)

func TestConnectTCPStops(t *testing.T) {
	for _, reason := range []string{"quit", "peer disconnect", "cancel", "input EOF"} {
		t.Run(reason, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			source, keyboard := io.Pipe()
			defer source.Close()
			defer keyboard.Close()
			input := console.NewInput(ctx, source)
			conn, peer := net.Pipe()
			defer conn.Close()
			defer peer.Close()
			done := make(chan struct{})
			go func() {
				defer close(done)
				ConnectTCP(ctx, conn, bufio.NewReader(conn), input)
			}()
			switch reason {
			case "quit":
				if _, err := io.WriteString(keyboard, "q\n"); err != nil {
					t.Fatal(err)
				}
			case "peer disconnect":
				peer.Close()
			case "cancel":
				cancel()
			case "input EOF":
				keyboard.Close()
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				cancel()
				conn.Close()
				t.Fatal("session did not stop")
			}
		})
	}
}

type observedConn struct {
	net.Conn
	writing chan struct{}
}

func (c *observedConn) Write(p []byte) (int, error) {
	close(c.writing)
	return c.Conn.Write(p)
}

func TestConnectTCPCancelsBlockedWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source, keyboard := io.Pipe()
	defer source.Close()
	defer keyboard.Close()
	input := console.NewInput(ctx, source)
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	conn := &observedConn{Conn: local, writing: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ConnectTCP(ctx, conn, bufio.NewReader(conn), input)
	}()
	if _, err := io.WriteString(keyboard, "hello\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-conn.writing:
	case <-time.After(2 * time.Second):
		t.Fatal("write did not start")
	}
	// The peer never reads, so cancellation must interrupt the write too.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session is stuck writing")
	}
}
