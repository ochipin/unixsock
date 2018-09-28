package unixsock

import (
	"fmt"
	"testing"
	"time"
)

func Test__SUCCESS_PATERN(t *testing.T) {
	sock := &Server{}

	// Called on error
	sock.Failed = func(err error) {
		// Output errorlog
		t.Fatal(err)
	}
	// Called on success
	sock.Success = func(mes_from_client []byte, conn Conn) {
		// Output : Hi, data please.
		if string(mes_from_client) != "Hi, data please." {
			t.Fatal("sock.Success: error")
		}
		fmt.Println(string(mes_from_client))
		// Return message to client
		conn.Write([]byte("Hello!!"))
	}
	// Socket file path to create
	sock.SocketFile = "server.sock"

	go func() {
		// Run server
		if err := sock.Run(); err != nil {
			t.Fatal(err)
		}
	}()

	// Wait: 10ms
	time.Sleep(10 * time.Millisecond)

	// Send message server
	message, err := Send("server.sock", []byte("Hi, data please."))
	if err != nil {
		t.Fatal("error:", err)
		return
	}

	if message != "Hello!!" {
		t.Fatal(message)
	}

	// Close
	sock.Close()
	time.Sleep(10 * time.Millisecond)

	fmt.Println(message)
}
