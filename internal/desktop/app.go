// Package desktop adapts usecase operations to the Wails frontend. It deliberately
// has no GUI imports, allowing its lifecycle to be tested without a display.
package desktop

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Weit145/local-sender/internal/connect"
	"github.com/Weit145/local-sender/internal/discovery"
	"github.com/Weit145/local-sender/internal/usecase"
)

type Message struct {
	ID           uint64 `json:"id"`
	ConnectionID string `json:"connectionID"`
	Text         string `json:"text"`
	Outgoing     bool   `json:"outgoing"`
	Time         string `json:"time"`
}

type State struct {
	Revision    uint64                      `json:"revision"`
	Generation  uint64                      `json:"generation"`
	Hostname    string                      `json:"hostname"`
	Platform    string                      `json:"platform"`
	Running     bool                        `json:"running"`
	Error       string                      `json:"error"`
	Settings    Settings                    `json:"settings"`
	Interfaces  []usecase.InterfaceAddress  `json:"interfaces"`
	Devices     []usecase.Device            `json:"devices"`
	Connections []usecase.ConnectionInfo    `json:"connections"`
	Requests    []usecase.ConnectionRequest `json:"requests"`
	Messages    []Message                   `json:"messages"`
	Searching   bool                        `json:"searching"`
	Connecting  []string                    `json:"connecting"`
}

type operation struct {
	id     uint64
	cancel context.CancelFunc
}

type App struct {
	lifecycle     sync.Mutex
	mu            sync.Mutex
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
	serviceCancel context.CancelFunc
	service       *usecase.Usecase
	path          string
	settings      Settings
	emit          func(State)
	closed        bool
	initialized   bool
	generation    uint64
	revision      uint64
	nextID        uint64
	operations    map[string]operation
	messages      []Message
	lastError     string
	hostname      string
}

func New(path string, emit func(State)) *App {
	settings, err := loadSettings(path)
	ctx, cancel := context.WithCancel(context.Background())
	hostname, _ := os.Hostname()
	a := &App{ctx: ctx, cancel: cancel, path: path, settings: settings, emit: emit, hostname: hostname, operations: make(map[string]operation), messages: make([]Message, 0)}
	if err != nil {
		a.settings = DefaultSettings()
		a.lastError = "Не удалось прочитать настройки: " + err.Error()
	}
	return a
}

// Initialize is called after the frontend has subscribed to events.
func (a *App) Initialize() State {
	a.lifecycle.Lock()
	defer a.lifecycle.Unlock()
	a.mu.Lock()
	initialized, closed := a.initialized, a.closed
	a.initialized = true
	a.mu.Unlock()
	if !initialized && !closed {
		if err := a.start(); err != nil {
			a.mu.Lock()
			a.lastError = err.Error()
			a.mu.Unlock()
		}
	}
	return a.Snapshot()
}

// start and stop require lifecycle to be held. Each generation owns a fresh
// event stream, so old events cannot overwrite the state after settings change.
func (a *App) start() error {
	a.mu.Lock()
	settings := a.settings
	ctx, cancel := context.WithCancel(a.ctx)
	service := usecase.NewUsecase(connect.NewConnect(settings.TCPAddress), discovery.NewDiscover(settings.TCPAddress, settings.UDPAddress, settings.BroadcastAddress, time.Duration(settings.SearchTimeout)*time.Second))
	a.service, a.serviceCancel = service, cancel
	a.generation++
	a.messages = make([]Message, 0)
	a.wg.Add(1)
	a.mu.Unlock()
	go func() {
		defer a.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-service.Events():
				a.mu.Lock()
				if a.service != service {
					a.mu.Unlock()
					continue
				}
				if event.Type == usecase.EventMessageReceived {
					a.appendMessage(event.Connection.ID, event.Message, false)
				}
				if event.Type == usecase.EventError {
					a.lastError = event.Error
				}
				a.mu.Unlock()
				a.publish()
			}
		}
	}()
	if err := service.Start(ctx); err != nil {
		cancel()
		return fmt.Errorf("не удалось запустить сеть: %w", err)
	}
	return nil
}

func (a *App) stop() {
	a.mu.Lock()
	service, cancel := a.service, a.serviceCancel
	for _, op := range a.operations {
		op.cancel()
	}
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if service != nil {
		service.Stop()
	}
}

func (a *App) Close() {
	a.lifecycle.Lock()
	defer a.lifecycle.Unlock()
	a.mu.Lock()
	a.closed = true
	a.cancel()
	a.mu.Unlock()
	a.stop()
	a.wg.Wait()
}

func (a *App) Snapshot() State {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.revision++
	result := State{Revision: a.revision, Generation: a.generation, Hostname: a.hostname, Platform: runtime.GOOS, Settings: a.settings, Error: a.lastError, Messages: slices.Clone(a.messages), Connecting: make([]string, 0), Interfaces: make([]usecase.InterfaceAddress, 0), Devices: make([]usecase.Device, 0), Connections: make([]usecase.ConnectionInfo, 0), Requests: make([]usecase.ConnectionRequest, 0)}
	if a.service != nil {
		result.Running = a.service.Running()
		if interfaces, err := a.service.ListInterfaces(); err == nil {
			result.Interfaces = interfaces
		}
		result.Devices = a.service.ListDevices()
		result.Connections = a.service.ListConnections()
		result.Requests = a.service.ListPendingRequests()
	}
	for key := range a.operations {
		if key == "search" {
			result.Searching = true
		}
		if addr, ok := strings.CutPrefix(key, "connect:"); ok {
			result.Connecting = append(result.Connecting, addr)
		}
	}
	sort.Strings(result.Connecting)
	return result
}

func (a *App) publish() {
	state := a.Snapshot()
	if a.emit != nil {
		a.emit(state)
	}
}

func (a *App) begin(key string) (*usecase.Usecase, context.Context, func(), error) {
	a.mu.Lock()
	if a.closed || a.service == nil || !a.service.Running() {
		a.mu.Unlock()
		return nil, nil, nil, usecase.ErrNotRunning
	}
	if _, exists := a.operations[key]; exists {
		a.mu.Unlock()
		return nil, nil, nil, fmt.Errorf("операция уже выполняется")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	a.nextID++
	id := a.nextID
	a.operations[key] = operation{id: id, cancel: cancel}
	service := a.service
	a.wg.Add(1)
	a.mu.Unlock()
	a.publish()
	finish := func() {
		cancel()
		a.mu.Lock()
		if op, ok := a.operations[key]; ok && op.id == id {
			delete(a.operations, key)
		}
		a.mu.Unlock()
		a.publish()
		a.wg.Done()
	}
	return service, ctx, finish, nil
}

func (a *App) Discover(ip string) error {
	service, ctx, finish, err := a.begin("search")
	if err != nil {
		return err
	}
	defer finish()
	_, err = service.DiscoverDevices(ctx, ip)
	return err
}
func (a *App) CancelSearch()             { a.cancelOperation("search") }
func (a *App) CancelConnect(addr string) { a.cancelOperation("connect:" + addr) }
func (a *App) cancelOperation(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if op, ok := a.operations[key]; ok {
		op.cancel()
	}
}
func (a *App) Connect(addr string) (usecase.ConnectionInfo, error) {
	service, ctx, finish, err := a.begin("connect:" + addr)
	if err != nil {
		return usecase.ConnectionInfo{}, err
	}
	defer finish()
	return service.Connect(ctx, addr)
}
func (a *App) Accept(id string) error {
	service, _, finish, err := a.begin("decision:" + id)
	if err != nil {
		return err
	}
	defer finish()
	return service.AcceptConnection(id)
}
func (a *App) Reject(id string) error {
	service, _, finish, err := a.begin("decision:" + id)
	if err != nil {
		return err
	}
	defer finish()
	return service.RejectConnection(id)
}
func (a *App) Disconnect(id string) error {
	service, _, finish, err := a.begin("disconnect:" + id)
	if err != nil {
		return err
	}
	defer finish()
	return service.Disconnect(id)
}
func (a *App) Send(id, text string) error {
	service, ctx, finish, err := a.begin("send:" + id)
	if err != nil {
		return err
	}
	defer finish()
	if err := service.SendMessage(ctx, id, text); err != nil {
		return err
	}
	a.mu.Lock()
	if a.service == service {
		a.appendMessage(id, text, true)
	}
	a.mu.Unlock()
	return nil
}
func (a *App) appendMessage(id, text string, outgoing bool) {
	a.nextID++
	a.messages = append(a.messages, Message{ID: a.nextID, ConnectionID: id, Text: text, Outgoing: outgoing, Time: time.Now().Format(time.RFC3339Nano)})
	if len(a.messages) > 1000 {
		a.messages = slices.Clone(a.messages[len(a.messages)-1000:])
	}
}

// SaveSettings rolls back both networking and settings if binding or persistence
// fails. Saving unchanged network settings does not disconnect sessions.
func (a *App) SaveSettings(settings Settings) error {
	if err := settings.validate(); err != nil {
		return err
	}
	a.lifecycle.Lock()
	defer a.lifecycle.Unlock()
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return usecase.ErrNotRunning
	}
	old := a.settings
	networkChanged := settings.TCPAddress != old.TCPAddress || settings.UDPAddress != old.UDPAddress || settings.BroadcastAddress != old.BroadcastAddress || settings.SearchTimeout != old.SearchTimeout || a.service == nil || !a.service.Running()
	a.mu.Unlock()
	if networkChanged {
		a.stop()
	}
	a.mu.Lock()
	a.settings = settings
	a.mu.Unlock()
	var err error
	if networkChanged {
		err = a.start()
	}
	if err == nil {
		err = saveSettings(a.path, settings)
	}
	if err != nil {
		if networkChanged {
			a.stop()
		}
		a.mu.Lock()
		a.settings = old
		a.mu.Unlock()
		if networkChanged {
			if rollback := a.start(); rollback != nil {
				err = fmt.Errorf("%v; не удалось восстановить сеть: %w", err, rollback)
			}
		}
		a.mu.Lock()
		a.lastError = err.Error()
		a.mu.Unlock()
		a.publish()
		return err
	}
	a.mu.Lock()
	a.initialized = true
	a.lastError = ""
	a.mu.Unlock()
	a.publish()
	return nil
}
