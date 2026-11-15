package usecase

import (
	"context"
	"sync"

	"github.com/Weit145/local-sender/internal/connect"
	"github.com/Weit145/local-sender/internal/usecase/port"
)

type run struct {
	ctx    context.Context
	cancel context.CancelFunc
	fail   context.CancelCauseFunc
	wg     sync.WaitGroup
}
type session struct {
	ctx      context.Context
	finished chan struct{}
	info     ConnectionInfo
	conn     *connect.Connection
	cancel   context.CancelFunc
	ready    chan struct{}
	readyErr error
	decided  bool
}

type Usecase struct {
	con       port.Connect
	disc      port.Discover
	lifecycle sync.Mutex
	mu        sync.Mutex
	current   *run
	sessions  map[string]*session
	devices   []Device
	searching bool
	nextID    uint64
	events    chan Event
}

func NewUsecase(con port.Connect, disc port.Discover) *Usecase {
	return &Usecase{con: con, disc: disc, sessions: make(map[string]*session), events: make(chan Event, 256)}
}

func (u *Usecase) Events() <-chan Event { return u.events }

// Prefer publishing a final state when there is space, even after cancellation.
func (u *Usecase) emit(ctx context.Context, event Event) {
	select {
	case u.events <- event:
		return
	default:
	}
	select {
	case u.events <- event:
	case <-ctx.Done():
	}
}
