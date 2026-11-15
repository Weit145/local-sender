package usecase

import "errors"

var (
	ErrNotRunning       = errors.New("usecase is not running")
	ErrNotFound         = errors.New("connection not found")
	ErrInvalidState     = errors.New("invalid connection state")
	ErrAlreadyConnected = errors.New("connection already exists")
	ErrSearchInProgress = errors.New("device search already in progress")
)

type ConnectionStatus string

const (
	StatusConnecting       ConnectionStatus = "connecting"
	StatusAwaitingApproval ConnectionStatus = "awaiting_approval"
	StatusConnected        ConnectionStatus = "connected"
	StatusDisconnected     ConnectionStatus = "disconnected"
)

type Device struct {
	IP      string
	Address string
}
type InterfaceAddress struct {
	Name    string
	IP      string
	Network string
}
type ConnectionInfo struct {
	ID       string
	Address  string
	Incoming bool
	Status   ConnectionStatus
}
type ConnectionRequest struct {
	ID      string
	Address string
}

type EventType string

const (
	EventConnectionRequested EventType = "connection_requested"
	EventConnectionChanged   EventType = "connection_changed"
	EventMessageReceived     EventType = "message_received"
	EventDevicesChanged      EventType = "devices_changed"
	EventError               EventType = "error"
)

type Event struct {
	Type       EventType
	Connection ConnectionInfo
	Message    string
	Devices    []Device
	Error      string
}
