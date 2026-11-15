package discovery

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestFindServersSavesReplyAndHonorsTimeout(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	go func() {
		buffer := make([]byte, 1024)
		_, peer, err := server.ReadFromUDP(buffer)
		if err == nil {
			server.WriteToUDP([]byte("MY_TCP_PORT::8080"), peer)
		}
	}()
	d := NewDiscover(":8080", ":9999", server.LocalAddr().String(), 100*time.Millisecond)
	done := make(chan struct{})
	go func() { d.FindServers(context.Background(), net.IPv4(127, 0, 0, 1)); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("search ignored configured timeout")
	}
	servers := d.GetServers()
	if servers["127.0.0.1"] != "MY_TCP_PORT::8080" {
		t.Fatalf("reply not saved: %v", servers)
	}
	delete(servers, "127.0.0.1")
	if len(d.GetServers()) != 1 {
		t.Fatal("caller modified internal map")
	}
}
