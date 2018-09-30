UNIX DOMAIN ソケット通信ライブラリ
===

UNIX DOMAIN ソケット通信をするライブラリです。

### Server
```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ochipin/unixsock"
)

func main() {
	sock := &unixsock.Server{}

	// Called on error
	sock.Failed = func(err error) {
		// Output errorlog
		fmt.Println("error:", err)
	}
	// Called on success
	sock.Success = func(mes_from_client []byte, conn unixsock.Conn) {
		// Output : Hi, data please.
		fmt.Println(string(mes_from_client))
		// Return message to client
		conn.Write([]byte("Hello!!"))
	}
	// Socket file path to create
	sock.SocketFile = "server.sock"

	// Signal Handling
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		for {
			switch <-sig {
			case syscall.SIGTERM, os.Interrupt:
				// Close
				sock.Close()
			}
		}
	}()

	// Run server
	if err := sock.Run(); err != nil {
		panic(err)
	}
}
```

### Client
```go
package main

import (
	"fmt"

	"github.com/ochipin/unixsock"
)

func main() {
	// Send message server
	message, err := unixsock.Send("server.sock", []byte("Hi, data please."))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	// Output : Hello!!
	fmt.Println(message)
}
```