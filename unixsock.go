package unixsock

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Send : サーバ側へメッセージを送信する
func Send(sockfile string, message []byte) (string, error) {
	// 接続開始
	conn, err := net.Dial("unix", sockfile)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	// メッセージをサーバ側へ送信
	if _, err := conn.Write(message); err != nil {
		return "", err
	}

	// メッセージの書き込みを終了
	unixconn, ok := conn.(*net.UnixConn)
	if !ok {
		return "", fmt.Errorf("net.UnixConn not type")
	}
	if err := unixconn.CloseWrite(); err != nil {
		return "", err
	}

	// メッセージ送信後、サーバ側から受信したデータを解析する
	data := make([]byte, 0)
	for {
		// 1024 byte 毎に受信する
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		// EOFの場合は、ループを抜ける
		if err != nil {
			if err != io.EOF {
				return "", err
			}
			break
		}
		buf = buf[:n]
		data = append(data, buf...)
	}

	// サーバから受信したデータを返却する
	return string(data), nil
}

// Conn : net.Conn のインターフェース
type Conn interface {
	Write([]byte) (int, error)
}

// Server : UNIX DOMAIN SOCKET サーバを構築する構造体
type Server struct {
	SocketFile string             // 生成するソケットファイルのパス
	Failed     func(error)        // 失敗時にコールされる関数
	Success    func([]byte, Conn) // 成功時にコールされる関数
	listen     net.Listener       // サーバ接続用 listen
	end        bool               // サーバ終了フラグ
	err        chan error         // エラーチャンネル
}

// Close : Server を終了させる
func (sock *Server) Close() {
	sock.end = true
	os.Remove(sock.SocketFile)
}

// Run : UNIX DOMAIN SOCKET サーバを起動する
func (sock *Server) Run() error {
	sock.err = make(chan error, 1)
	// Failed/Success 関数が定義されているか確認する
	if sock.Failed == nil {
		return fmt.Errorf("Failed() function undefiend")
	}
	if sock.Success == nil {
		return fmt.Errorf("Success() function undefined")
	}
	// SocketFile にパスが指定されているか確認する
	if sock.SocketFile == "" {
		return fmt.Errorf("SocketFile is nil")
	}

	// すでにサーバが立ち上がっている場合、すみやかに関数を復帰する
	if err := sock.Check(); err != nil {
		return err
	}

	// SocketFile に指定されている文字列が、/path/to/url/file.sock の場合、 /path/to/url を作成する
	dir, file := filepath.Split(sock.SocketFile)
	if file == "" {
		return fmt.Errorf("SocketFile is invalid value")
	}
	if dir != "" {
		os.MkdirAll(dir, 0755)
	}

	// この時点で、SocketFile が作成される
	var lerr error
	if sock.listen, lerr = net.Listen("unix", sock.SocketFile); lerr != nil {
		return lerr
	}
	defer sock.listen.Close()

	// SocketFileが何らかの理由により削除されていないか監視する
	go func() {
		lerr = sock.sockFileMonitor()
		sock.listen.Close()
		sock.err <- lerr
	}()

	// メッセージ受信
	go func() {
		for {
			// サーバ終了の合図がある場合、ループを抜ける
			if sock.end {
				break
			}

			// クライアントからのメッセージ待つ
			conn, err := sock.listen.Accept()
			if err != nil {
				// すでに、l.Close() されている場合、クライアントがアクセスした際に、ここでエラーとなる。
				// 再度受付するために、continue する
				continue
			}

			// l.Accept() 成功時は、別スレッドで処理を実施
			go func(conn net.Conn) {
				defer conn.Close()
				// クライアントから送信された内容を受信し、data へ格納する
				data := make([]byte, 0)
				for {
					// 128 byte 毎に受信する
					buf := make([]byte, 128)
					n, err := conn.Read(buf)
					// 受信エラーが発生した場合は、Failed() をコール
					if err != nil {
						if err != io.EOF {
							sock.Failed(err)
						}
						break
					}
					buf = buf[:n]
					data = append(data, buf...)
				}
				// メッセージ受信成功の場合は、Success() をコール
				sock.Success(data, conn)
			}(conn)
		}
	}()

	return <-sock.err
}

// Check : SocketFile が有効か否かを判定する
func (sock *Server) Check() error {
	// すでにサーバが立ち上がっている場合、すみやかに関数を復帰する
	conn, err := net.Dial("unix", sock.SocketFile)
	if err == nil {
		conn.Close()
		return fmt.Errorf("already in use '%s'", sock.SocketFile)
	}
	if err != nil {
		idx := strings.Index(err.Error(), "connection refused")
		// 接続ができない状態の場合、すでに使用されていないSocketFileということになる
		if idx != -1 {
			// 重要なファイルの可能性もあるため、一旦ファイルを読み込む
			_, err := ioutil.ReadFile(sock.SocketFile)
			// ファイルが読み込めてしまった場合は、エラーを返却する
			if err == nil {
				return fmt.Errorf("already in use '%s'", sock.SocketFile)
			}
			// 読み込めない場合は、すでに使用されていないSocketFileとみなし、削除する
			os.Remove(sock.SocketFile)
		}
	}
	return nil
}

// SocketFile を監視する
func (sock *Server) sockFileMonitor() (err error) {
	tick := time.NewTicker(1 * time.Second)
	for {
		select {
		// 1 毎に SocketFile が存在するか確認する
		case <-tick.C:
			if sock.end {
				return nil
			}
			// SocketFile が存在する場合、何もしない
			if _, err = os.Stat(sock.SocketFile); err == nil {
				continue
			}
			// SocketFile に指定されている文字列が、/path/to/url/file.sock の場合、 /path/to/url を作成する
			dir, file := filepath.Split(sock.SocketFile)
			if file == "" {
				return fmt.Errorf("SocketFile is invalid value")
			}
			if dir != "" {
				os.MkdirAll(dir, 0755)
			}
			// SocketFile が存在しない場合、再度SocketFileを生成する
			sock.listen.Close()
			if sock.listen, err = net.Listen("unix", sock.SocketFile); err != nil {
				return err
			}
		}
	}
}
