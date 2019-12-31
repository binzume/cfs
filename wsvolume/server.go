package wsvolume

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/binzume/cfs/volume"
	"github.com/gorilla/websocket"
)

type WebsocketVolumeProvider struct {
	volume            volume.FS
	connected         bool
	reconnectInterval time.Duration
}

func NewWebsocketVolumeProvider(volume volume.FS) *WebsocketVolumeProvider {
	return &WebsocketVolumeProvider{
		volume:            volume,
		reconnectInterval: time.Second * 3,
	}
}

func (wp *WebsocketVolumeProvider) StartClient(connector websocketConnector) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := connector()
			if err != nil {
				log.Printf(" error: %v", err)
				return
			}
			wp.HandleSession(conn, "file")
			time.Sleep(wp.reconnectInterval)
		}
	}()
	return done
}

func (wp *WebsocketVolumeProvider) StartClientWithDefaultConnector(wsurl string) <-chan struct{} {
	var connector = func() (*websocket.Conn, error) {
		c, _, err := websocket.DefaultDialer.Dial(wsurl, nil)
		return c, err
	}
	return wp.StartClient(connector)
}

func (wp *WebsocketVolumeProvider) Terminate() {
}

func (v *WebsocketVolumeProvider) HandleRequest(w http.ResponseWriter, r *http.Request, header http.Header) error {
	conn, err := WSUpgrader.Upgrade(w, r, header)
	if err != nil {
		return err
	}
	go v.HandleSession(conn, "file")
	return nil
}

func (wp *WebsocketVolumeProvider) HandleSession(conn *websocket.Conn, target string) error {
	conn.WriteJSON(&map[string]interface{}{})
	wp.connected = true
	defer func() { wp.connected = false }()
	ConnectClient(wp.volume, conn, target) // TODO: refactoring
	return nil
}

func errorResponse(rid interface{}, msg string) *map[string]interface{} {
	return &map[string]interface{}{"error": msg, "rid": rid}
}

func readBlock(v volume.FS, path string, dst []byte, offset int64) (int, error) {
	f, err := v.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if r, ok := f.(io.ReaderAt); ok {
		return r.ReadAt(dst, offset)
	}
	return 0, err
}

func writeBlock(v volume.FS, path string, dst []byte, offset int64) (int, error) {
	f, err := v.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if r, ok := f.(io.WriterAt); ok {
		return r.WriteAt(dst, offset)
	}
	return 0, err
}

func fileOperation(v volume.FS, conn *websocket.Conn) {
	for {
		var op map[string]json.Number
		err := conn.ReadJSON(&op)
		if err != nil {
			return
		}
		log.Println("op:", op)
		rid := op["rid"]
		switch op["op"].String() {
		case "stat":
			st, err := v.Stat(op["path"].String())
			if err != nil {
				conn.WriteJSON(errorResponse(rid, "readdir error"))
				continue
			}
			conn.WriteJSON(&map[string]interface{}{"rid": rid, "stat": st})
		case "read":
			l, _ := op["l"].Int64()
			p, _ := op["p"].Int64()
			b := make([]byte, l)
			len, _ := readBlock(v, op["path"].String(), b, p)
			conn.WriteMessage(websocket.BinaryMessage, b[:len])
		case "write":
			p, _ := op["p"].Int64()
			b := []byte(op["b"].String())
			len, _ := writeBlock(v, op["path"].String(), b, p)
			conn.WriteJSON(&map[string]interface{}{"rid": rid, "l": len})
		case "remove":
			_ = v.Remove(op["path"].String())
			conn.WriteJSON(&map[string]interface{}{"rid": rid})
		case "files":
			files, err := v.ReadDir(op["path"].String())
			if err != nil {
				conn.WriteJSON(errorResponse(rid, "readdir error"))
				continue
			}
			conn.WriteJSON(&map[string]interface{}{"rid": rid, "files": files})
		default:
			conn.WriteJSON(errorResponse(rid, "unknown operation"))
		}
	}
}

func ConnectClient(v volume.FS, conn *websocket.Conn, target string) {
	log.Println("connect", target)
	var event map[string]string
	if target == "file" {
		fileOperation(v, conn)
	} else {
		err := conn.ReadJSON(&event)
		if err != nil {
			return
		}
		log.Println(event)
	}
	log.Println("disconnect")
}
