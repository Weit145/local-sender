package discovery

import (
	"net"
	"testing"
)

func TestSubnetBroadcast(t *testing.T) {
	for _, tc := range []struct{ cidr, want string }{
		{"192.168.1.42/24", "192.168.1.255"},
		{"192.168.2.42/23", "192.168.3.255"},
		{"10.4.8.2/16", "10.4.255.255"},
		{"192.168.1.2/31", "<nil>"},
		{"192.168.1.2/32", "<nil>"},
		{"fe80::1/64", "<nil>"},
	} {
		t.Run(tc.cidr, func(t *testing.T) {
			ip, network, err := net.ParseCIDR(tc.cidr)
			if err != nil {
				t.Fatal(err)
			}
			network.IP = ip
			if got := subnetBroadcast(network).String(); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestExplicitDiscoveryTarget(t *testing.T) {
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.42"), Port: 12345}
	targets, err := discoveryTargets(addr)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].remote.String() != "192.168.1.42:12345" {
		t.Fatalf("explicit server address was changed: %+v", targets)
	}
}
