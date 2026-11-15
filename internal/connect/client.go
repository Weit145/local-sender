package connect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

var ErrRejected = errors.New("connection rejected")
var ErrDuplicate = errors.New("connection already exists")

// ClientConnect runs until the connection ends. Ready reports handshake completion.
func (c *Connector) ClientConnect(ctx context.Context, addr string, connection *Connection) (err error) {
	defer func() { connection.finish(err) }()
	if !c.addConnect(addr, connection) {
		return ErrDuplicate
	}
	defer c.removeConnect(addr, connection)
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	stop := watchConnection(ctx, conn, connection)
	defer stop()
	reader := bufio.NewReader(conn)
	approval, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	switch strings.TrimSpace(approval) {
	case "ACCEPTED":
	case "REJECTED":
		return ErrRejected
	default:
		return fmt.Errorf("unexpected approval: %q", approval)
	}
	connection.ready(nil)
	return c.connectTCP(ctx, conn, reader, connection)
}
