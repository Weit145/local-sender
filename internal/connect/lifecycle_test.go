package connect

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"
)

func testConnection() *Connection {
	return &Connection{Send: make(chan []byte), Recv: make(chan []byte), Exit: make(chan struct{})}
}

func TestSessionStopsWithUnreadMessage(t *testing.T) {
	c := NewConnect("")
	local, remote := net.Pipe()
	defer remote.Close()
	connection := testConnection()
	done := make(chan struct{})
	go func() { c.connectTCP(context.Background(), local, bufio.NewReader(local), connection); close(done) }()
	if _, err := remote.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	connection.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("session blocked on unread message")
	}
}

func TestDeleteConnectWithoutReceiver(t *testing.T) {
	c := NewConnect("")
	connection := testConnection()
	c.AddConnect("peer", connection)
	done := make(chan struct{})
	go func() { c.DeleteConnect("peer"); c.DeleteConnect("peer"); connection.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delete blocked")
	}
	if c.CheckConnect("peer") {
		t.Fatal("connection still registered")
	}
	select {
	case <-connection.Exit:
	default:
		t.Fatal("connection not stopped")
	}
}

func TestOldSessionDoesNotRemoveReplacement(t *testing.T) {
	c := NewConnect("")
	old, replacement := testConnection(), testConnection()
	c.AddConnect("peer", old)
	c.DeleteConnect("peer")
	c.AddConnect("peer", replacement)
	c.removeConnect("peer", old)
	if !c.CheckConnect("peer") {
		t.Fatal("replacement removed")
	}
}

func TestClientRemovesRejectedConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.Write([]byte("REJECTED\n"))
	}()
	c := NewConnect("")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	addr := listener.Addr().String()
	c.ClientConnect(ctx, addr, testConnection())
	if c.CheckConnect(addr) {
		t.Fatal("rejected connection still registered")
	}
}

func TestServerRemovesClosedConnection(t *testing.T) {
	c := NewConnect("")
	local, remote := net.Pipe()
	defer remote.Close()
	connection := testConnection()
	connection.RemoteAddr = "peer"
	c.AddConnect("peer", connection)
	done := make(chan struct{})
	go func() { c.serverConnect(context.Background(), local, connection); close(done) }()
	connection.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
	if c.CheckConnect("peer") {
		t.Fatal("closed server connection still registered")
	}
}
