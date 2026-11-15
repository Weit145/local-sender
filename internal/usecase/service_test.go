package usecase

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Weit145/local-sender/internal/connect"
	"github.com/Weit145/local-sender/internal/discovery"
)

func freeAddress(t *testing.T, network string) string {
	t.Helper()
	if network == "udp" {
		conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := conn.LocalAddr().String()
		conn.Close()
		return addr
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	return addr
}

func testService(t *testing.T, tcp, udp, destination string) *Usecase {
	t.Helper()
	u := NewUsecase(connect.NewConnect(tcp), discovery.NewDiscover(tcp, udp, destination, 80*time.Millisecond))
	if err := u.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(u.Stop)
	return u
}

func awaitEvent(t *testing.T, u *Usecase, kind EventType, status ConnectionStatus) Event {
	t.Helper()
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case event := <-u.Events():
			if event.Type == kind && (status == "" || event.Connection.Status == status) {
				return event
			}
		case <-timeout.C:
			t.Fatalf("timed out waiting for %s %s", kind, status)
		}
	}
}

type connectResult struct {
	info ConnectionInfo
	err  error
}

func connectAsync(ctx context.Context, u *Usecase, addr string) <-chan connectResult {
	result := make(chan connectResult, 1)
	go func() { info, err := u.Connect(ctx, addr); result <- connectResult{info, err} }()
	return result
}
func awaitResult(t *testing.T, result <-chan connectResult) connectResult {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("Connect blocked")
		return connectResult{}
	}
}

func TestTwoInstancesDiscoveryMessagingAndReconnect(t *testing.T) {
	tcpA, udpA := freeAddress(t, "tcp"), freeAddress(t, "udp")
	a := testService(t, tcpA, udpA, udpA)
	b := testService(t, freeAddress(t, "tcp"), freeAddress(t, "udp"), udpA)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	devices, err := b.DiscoverDevices(ctx, "127.0.0.1")
	if err != nil || len(devices) != 1 || devices[0].Address != tcpA {
		t.Fatalf("discovery: %v %v", devices, err)
	}
	devices[0].Address = "changed"
	if b.ListDevices()[0].Address != tcpA {
		t.Fatal("mutable device snapshot")
	}
	result := connectAsync(ctx, b, tcpA)
	request := awaitEvent(t, a, EventConnectionRequested, "")
	if len(a.ListPendingRequests()) != 1 || request.Connection.Address == "" {
		t.Fatal("missing pending request/address")
	}
	if err := a.AcceptConnection(request.Connection.ID); err != nil {
		t.Fatal(err)
	}
	connected := awaitResult(t, result)
	if connected.err != nil || connected.info.Status != StatusConnected {
		t.Fatalf("connect: %+v", connected)
	}
	if _, err := b.Connect(ctx, tcpA); !errors.Is(err, ErrAlreadyConnected) {
		t.Fatalf("duplicate: %v", err)
	}
	if err := b.SendMessage(ctx, connected.info.ID, "hello мир"); err != nil {
		t.Fatal(err)
	}
	if event := awaitEvent(t, a, EventMessageReceived, ""); event.Message != "hello мир" {
		t.Fatalf("message: %q", event.Message)
	}
	if err := a.SendMessage(ctx, request.Connection.ID, "reply"); err != nil {
		t.Fatal(err)
	}
	if event := awaitEvent(t, b, EventMessageReceived, ""); event.Message != "reply" {
		t.Fatalf("reply: %q", event.Message)
	}
	if err := b.Disconnect(connected.info.ID); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, b, EventConnectionChanged, StatusDisconnected)
	awaitEvent(t, a, EventConnectionChanged, StatusDisconnected)
	result = connectAsync(ctx, b, tcpA)
	request = awaitEvent(t, a, EventConnectionRequested, "")
	if err := a.RejectConnection(request.Connection.ID); err != nil {
		t.Fatal(err)
	}
	if rejected := awaitResult(t, result); !errors.Is(rejected.err, connect.ErrRejected) {
		t.Fatalf("rejection: %v", rejected.err)
	}
	awaitEvent(t, b, EventConnectionChanged, StatusDisconnected)
}

func TestCancelPendingConnectAndRestart(t *testing.T) {
	addr := freeAddress(t, "tcp")
	a := testService(t, addr, freeAddress(t, "udp"), "127.0.0.1:9999")
	b := testService(t, freeAddress(t, "tcp"), freeAddress(t, "udp"), "127.0.0.1:9999")
	ctx, cancel := context.WithCancel(context.Background())
	result := connectAsync(ctx, b, addr)
	awaitEvent(t, a, EventConnectionRequested, "")
	cancel()
	if got := awaitResult(t, result); !errors.Is(got.err, context.Canceled) {
		t.Fatalf("cancellation: %v", got.err)
	}
	awaitEvent(t, a, EventConnectionChanged, StatusDisconnected)
	b.Stop()
	b.Stop()
	if err := b.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := b.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStopWithFullEventQueue(t *testing.T) {
	addr := freeAddress(t, "tcp")
	a := testService(t, addr, freeAddress(t, "udp"), "127.0.0.1:9999")
	b := testService(t, freeAddress(t, "tcp"), freeAddress(t, "udp"), "127.0.0.1:9999")
	for len(a.events) < cap(a.events) {
		a.events <- Event{Type: EventDevicesChanged}
	}
	result := connectAsync(context.Background(), b, addr)
	// Wait for the incoming request to be registered while its event is blocked.
	deadline := time.After(3 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
waiting:
	for {
		if len(a.ListPendingRequests()) > 0 {
			break waiting
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatal("incoming request not registered")
		}
	}
	// The pending request's event cannot be consumed, but Stop must still finish.
	done := make(chan struct{})
	go func() { a.Stop(); b.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Stop blocked on event queue")
	}
	awaitResult(t, result)
}

func TestFailedStartupReleasesTCPListener(t *testing.T) {
	udp, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	tcp := freeAddress(t, "tcp")
	u := NewUsecase(connect.NewConnect(tcp), discovery.NewDiscover(tcp, udp.LocalAddr().String(), udp.LocalAddr().String(), time.Second))
	if err := u.Start(context.Background()); err == nil {
		u.Stop()
		t.Fatal("expected bind error")
	} else if errors.Is(err, context.Canceled) {
		t.Fatalf("bind error hidden by cancellation: %v", err)
	}
	listener, err := net.Listen("tcp", tcp)
	if err != nil {
		t.Fatalf("TCP listener leaked: %v", err)
	}
	listener.Close()
	u.Stop()
}
