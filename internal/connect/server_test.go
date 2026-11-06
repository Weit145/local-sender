package connect

import (
	"bufio"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestServerHandlesMultipleMessages(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()
	if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		NewServerConnect().Handler(server)
	}()
	reader := bufio.NewReader(client)
	for _, message := range []string{"hello", "привет"} {
		if _, err := fmt.Fprintln(client, message); err != nil {
			t.Fatal(err)
		}
		reply, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if reply != "Сообщение получено\n" {
			t.Fatalf("неожиданный ответ: %q", reply)
		}
	}
	client.Close()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("обработчик не завершился после отключения клиента")
	}
}
