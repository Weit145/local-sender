package port

import (
	"context"
	"github.com/Weit145/local-sender/internal/connect"
)

type Connect interface {
	ClientConnect(context.Context, string, *connect.Connection) error
	ListenServer(context.Context, chan<- *connect.Connection, chan<- error) error
}
