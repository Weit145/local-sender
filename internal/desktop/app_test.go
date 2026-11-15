package desktop

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func availableAddress(t *testing.T, network string) string {
	t.Helper()
	if network == "udp" {
		conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		return conn.LocalAddr().String()
	}
	conn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return conn.Addr().String()
}
func newTestApp(t *testing.T) *App {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	settings := DefaultSettings()
	settings.TCPAddress = availableAddress(t, "tcp")
	settings.UDPAddress = availableAddress(t, "udp")
	settings.BroadcastAddress = settings.UDPAddress
	if err := saveSettings(path, settings); err != nil {
		t.Fatal(err)
	}
	app := New(path, nil)
	t.Cleanup(app.Close)
	if state := app.Initialize(); !state.Running {
		t.Fatal(state.Error)
	}
	return app
}

func TestSettingsPersistWithoutUnnecessaryRestart(t *testing.T) {
	app := newTestApp(t)
	before := app.Snapshot()
	settings := before.Settings
	if err := app.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	after := app.Snapshot()
	if !after.Running || after.Generation != before.Generation {
		t.Fatal("unchanged settings restarted network")
	}
	loaded, err := loadSettings(app.path)
	if err != nil || loaded != settings {
		t.Fatalf("settings not persisted: %v %v", loaded, err)
	}
}
func TestSettingsRollbackAfterBindFailure(t *testing.T) {
	app := newTestApp(t)
	before := app.Snapshot()
	blocked, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocked.Close()
	settings := before.Settings
	settings.TCPAddress = blocked.Addr().String()
	if err := app.SaveSettings(settings); err == nil {
		t.Fatal("expected bind error")
	}
	state := app.Snapshot()
	if !state.Running || state.Settings != before.Settings {
		t.Fatalf("rollback failed: %+v", state)
	}
	saved, err := loadSettings(app.path)
	if err != nil || saved != before.Settings {
		t.Fatal("failed settings were persisted")
	}
}
func TestCancelSearchAndClose(t *testing.T) {
	app := newTestApp(t)
	done := make(chan error, 1)
	go func() { done <- app.Discover("127.0.0.1") }()
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
waiting:
	for {
		if app.Snapshot().Searching {
			break waiting
		}
		select {
		case <-ticker.C:
		case <-timeout:
			t.Fatal("search did not start")
		}
	}
	app.CancelSearch()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("search not canceled")
	}
	app.Close()
	app.Close()
	if state := app.Snapshot(); state.Running || state.Searching {
		t.Fatal("app still running")
	}
	if err := app.Discover("127.0.0.1"); err == nil {
		t.Fatal("operation allowed after shutdown")
	}
}
func TestMessageSnapshotIsIndependent(t *testing.T) {
	app := newTestApp(t)
	app.mu.Lock()
	app.appendMessage("1", "hello", true)
	app.mu.Unlock()
	state := app.Snapshot()
	state.Messages[0].Text = "changed"
	if app.Snapshot().Messages[0].Text != "hello" {
		t.Fatal("mutable message snapshot")
	}
}

// Old theme preferences must not reset a user's working network configuration.
func TestLegacyThemeSettingsPreserveNetworkConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	data := []byte(`{"tcpAddress":":8088","udpAddress":":9998","broadcastAddress":"192.168.1.255:9998","searchTimeout":7,"theme":"light"}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	settings, err := loadSettings(path)
	if err != nil {
		t.Fatal(err)
	}
	if settings.TCPAddress != ":8088" || settings.UDPAddress != ":9998" || settings.BroadcastAddress != "192.168.1.255:9998" || settings.SearchTimeout != 7 {
		t.Fatalf("legacy network settings lost: %+v", settings)
	}
}
