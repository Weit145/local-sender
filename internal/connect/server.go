package connect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// ListenServer reports successful binding through ready, then serves until canceled.
// It waits for all accepted sessions before returning.
func (c *Connector) ListenServer(ctx context.Context, incoming chan<- *Connection, ready chan<- error) error {
	listener, err := net.Listen("tcp", c.tcpAddr)
	ready <- err
	if err != nil {
		return err
	}
	defer listener.Close()
	stop := context.AfterFunc(ctx, func() { listener.Close() })
	defer stop()
	sessionsCtx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer wg.Wait()
	defer cancel()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		connection := NewConnection(conn.RemoteAddr().String())
		if !c.addConnect(connection.RemoteAddr, connection) {
			conn.Close()
			continue
		}
		select {
		case incoming <- connection:
			wg.Add(1)
			go func() { defer wg.Done(); c.serverConnect(sessionsCtx, conn, connection) }()
		case <-ctx.Done():
			conn.Close()
			c.removeConnect(connection.RemoteAddr, connection)
			connection.finish(ctx.Err())
			return ctx.Err()
		}
	}
}

func (c *Connector) serverConnect(ctx context.Context, conn net.Conn, connection *Connection) (err error) {
	defer func() { connection.finish(err) }()
	defer conn.Close()
	defer c.removeConnect(connection.RemoteAddr, connection)
	stop := watchConnection(ctx, conn, connection)
	defer stop()
	sessionCtx, cancel := context.WithCancel(ctx)
	readDone := startReader(sessionCtx, conn, bufio.NewReader(conn), connection)
	defer func() { cancel(); conn.Close(); <-readDone }()
	select {
	case err := <-readDone:
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-connection.Exit:
		return nil
	case message, ok := <-connection.Send:
		if !ok {
			return nil
		}
		if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return err
		}
		if string(message) != "ACCEPTED" {
			_, err := fmt.Fprintln(conn, "REJECTED")
			if err != nil {
				return err
			}
			return ErrRejected
		}
		if _, err := fmt.Fprintln(conn, "ACCEPTED"); err != nil {
			return err
		}
		if err := conn.SetWriteDeadline(time.Time{}); err != nil {
			return err
		}
	}
	connection.ready(nil)
	return exchangeTCP(sessionCtx, cancel, conn, connection, readDone)
}
