package desktop

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

type Settings struct {
	TCPAddress       string `json:"tcpAddress"`
	UDPAddress       string `json:"udpAddress"`
	BroadcastAddress string `json:"broadcastAddress"`
	SearchTimeout    int    `json:"searchTimeout"`
}

func DefaultSettings() Settings {
	return Settings{TCPAddress: ":8080", UDPAddress: ":9999", BroadcastAddress: "255.255.255.255:9999", SearchTimeout: 3}
}

func ConfigPath() (string, error) {
	if path := os.Getenv("LOCAL_SENDER_CONFIG"); path != "" {
		return path, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "local-sender", "settings.json"), nil
}

func (s Settings) validate() error {
	for name, addr := range map[string]string{"TCP": s.TCPAddress, "UDP": s.UDPAddress, "поиска": s.BroadcastAddress} {
		host, port, err := net.SplitHostPort(addr)
		n, portErr := strconv.Atoi(port)
		if err != nil || portErr != nil || n < 1 || n > 65535 {
			return fmt.Errorf("адрес %s должен содержать хост и порт от 1 до 65535", name)
		}
		if host != "" && net.ParseIP(host) == nil {
			return fmt.Errorf("в адресе %s укажите IP, а не имя хоста", name)
		}
		if name != "TCP" && host != "" && net.ParseIP(host).To4() == nil {
			return fmt.Errorf("для %s требуется IPv4", name)
		}
		if name == "поиска" && host == "" {
			return fmt.Errorf("укажите IPv4 для поиска")
		}
	}
	if s.SearchTimeout < 1 || s.SearchTimeout > 60 {
		return fmt.Errorf("время поиска должно быть от 1 до 60 секунд")
	}
	return nil
}

func loadSettings(path string) (Settings, error) {
	settings := DefaultSettings()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, err
	}
	return settings, settings.validate()
}

func saveSettings(path string, settings Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
