package connect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

type Connection struct {
	RemoteAddr string
	Ready      chan error
	Done       chan struct{}
	// Err may only be read after Done is closed.
	Err       error
	readyOnce sync.Once
	stopOnce  sync.Once
	Send      chan []byte
	Recv      chan []byte
	Exit      chan struct{}
}

func NewConnection(addr string) *Connection {
	return &Connection{RemoteAddr: addr, Send: make(chan []byte), Recv: make(chan []byte), Exit: make(chan struct{}), Ready: make(chan error, 1), Done: make(chan struct{})}
}

func (c *Connection) ready(err error) {
	c.readyOnce.Do(func() {
		if c.Ready != nil {
			c.Ready <- err
		}
	})
}

func (c *Connection) finish(err error) {
	c.Err = err
	if err == nil {
		c.ready(io.EOF)
	} else {
		c.ready(err)
	}
	if c.Done != nil {
		close(c.Done)
	}
}

// Close signals shutdown; callers must not close Exit directly.
func (c *Connection) Close() {
	c.stopOnce.Do(func() { close(c.Exit) })
}

type Connector struct {
	tcpAddr  string
	mx       sync.Mutex
	connects map[string]*Connection
}

func NewConnect(tcpAddr string) *Connector {
	return &Connector{
		tcpAddr:  tcpAddr,
		mx:       sync.Mutex{},
		connects: make(map[string]*Connection),
	}
}

func (c *Connector) CheckConnect(addr string) bool {
	c.mx.Lock()
	defer c.mx.Unlock()
	_, ok := c.connects[addr]
	return ok
}

func (c *Connector) AddConnect(addr string, conn *Connection) {
	c.addConnect(addr, conn)
}

func (c *Connector) addConnect(addr string, conn *Connection) bool {
	c.mx.Lock()
	defer c.mx.Unlock()
	if _, ok := c.connects[addr]; ok {
		return false
	}
	c.connects[addr] = conn
	return true
}

func (c *Connector) removeConnect(addr string, conn *Connection) {
	c.mx.Lock()
	defer c.mx.Unlock()
	if c.connects[addr] == conn {
		delete(c.connects, addr)
	}
}

func (c *Connector) DeleteConnect(addr string) {
	c.mx.Lock()
	conn := c.connects[addr]
	delete(c.connects, addr)
	c.mx.Unlock()
	if conn != nil {
		conn.Close()
	}
}

// watchConnection closes the socket even when a read or write is blocked.
func watchConnection(ctx context.Context, conn net.Conn, connection *Connection) func() {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-connection.Exit:
			conn.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

func (c *Connector) connectTCP(ctx context.Context, conn net.Conn, reader *bufio.Reader, connection *Connection) error {
	sessionCtx, cancel := context.WithCancel(ctx)
	readDone := startReader(sessionCtx, conn, reader, connection)
	return exchangeTCP(sessionCtx, cancel, conn, connection, readDone)
}

func startReader(ctx context.Context, conn net.Conn, reader *bufio.Reader, connection *Connection) <-chan error {
	done := make(chan error, 1)
	go func() { done <- readTCP(ctx, conn, reader, connection.Recv); close(done) }()
	return done
}

// exchangeTCP also accepts a reader started while awaiting approval, so a peer
// disconnecting during approval is noticed without concurrent socket readers.
func exchangeTCP(ctx context.Context, cancel context.CancelFunc, conn net.Conn, connection *Connection, readDone <-chan error) error {
	writeDone := make(chan error, 1)
	go func() { writeDone <- writeTCP(ctx, conn, connection.Send); close(writeDone) }()
	var err error
	readFinished, writeFinished := false, false
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-connection.Exit:
	case err = <-readDone:
		readFinished = true
	case err = <-writeDone:
		writeFinished = true
	}
	cancel()
	conn.Close()
	if !readFinished {
		<-readDone
	}
	if !writeFinished {
		<-writeDone
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func readTCP(ctx context.Context, conn net.Conn, reader *bufio.Reader, recv chan<- []byte) error {
	for {
		text, err := reader.ReadBytes('\n')
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if len(text) > 0 {
			select {
			case recv <- text:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if err != nil {
			return err
		}
	}
}

func writeTCP(ctx context.Context, conn net.Conn, send <-chan []byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case text, ok := <-send:
			if !ok {
				return nil
			}
			if _, err := fmt.Fprintln(conn, string(text)); err != nil {
				return err
			}
		}
	}
}
