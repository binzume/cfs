package wsvolume

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
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

func (wp *WebsocketVolumeProvider) StartClient(connector websocketConnector) (<-chan struct{}, error) {
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
	return done, nil
}

func (wp *WebsocketVolumeProvider) StartClientWithDefaultConnector(wsurl string) (<-chan struct{}, error) {
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

	log.Println("connect", target)
	c := &wsVolumeProviderConn{v: wp.volume, conn: conn}
	c.handleFileCommands()
	log.Println("disconnect")
	return nil
}

type wsVolumeProviderConn struct {
	v    volume.FS
	conn *websocket.Conn
}

func (c *wsVolumeProviderConn) readBlock(path string, dst []byte, offset int64) (int, error) {
	f, err := c.v.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if r, ok := f.(io.ReaderAt); ok {
		return r.ReadAt(dst, offset)
	}
	return 0, err
}

func (c *wsVolumeProviderConn) writeBlock(path string, data []byte, offset int64) (int, error) {
	f, err := c.v.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if r, ok := f.(io.WriterAt); ok {
		return r.WriteAt(data, offset)
	}
	return 0, err
}

func (c *wsVolumeProviderConn) response(rid interface{}, data interface{}) error {
	if data == nil {
		return c.conn.WriteJSON(&map[string]interface{}{"rid": rid})
	}
	return c.conn.WriteJSON(&map[string]interface{}{"rid": rid, "data": data})
}

func (c *wsVolumeProviderConn) errorResponse(rid interface{}, err error, op string) error {
	var msg string
	if os.IsNotExist(err) {
		msg = "noent"
	} else {
		msg = op + " error"
	}
	return c.conn.WriteJSON(&map[string]interface{}{"error": msg, "rid": rid})
}

func (c *wsVolumeProviderConn) readCommand() (map[string]json.Number, []byte, error) {
	mt, msg, err := c.conn.ReadMessage()
	if err != nil {
		return nil, nil, err
	}
	var op map[string]json.Number
	switch mt {
	case websocket.TextMessage:
		if err := json.Unmarshal(msg, &op); err != nil {
			return nil, nil, err
		}
		return op, nil, err
	case websocket.BinaryMessage:
		sz := binary.LittleEndian.Uint32(msg[4:])
		if err := json.Unmarshal(msg[8:8+sz], &op); err != nil {
			return nil, nil, err
		}
		return op, msg[8+sz:], err
	default:
		return nil, nil, fmt.Errorf("invalid message type")
	}
}

func (c *wsVolumeProviderConn) handleFileCommands() {
	for {
		cmd, data, err := c.readCommand()
		if err != nil {
			return
		}
		log.Print("op:", cmd["op"], cmd["path"])
		rid := cmd["rid"]
		op := cmd["op"].String()
		switch op {
		case "stat":
			st, err := c.v.Stat(cmd["path"].String())
			if err != nil {
				c.errorResponse(rid, err, op)
			} else {
				c.response(rid, st)
			}
		case "read":
			l, _ := cmd["l"].Int64()
			p, _ := cmd["p"].Int64()
			b := make([]byte, l+8)
			ridint, _ := rid.Int64()

			len, err := c.readBlock(cmd["path"].String(), b[8:], p)
			if err != nil {
				c.errorResponse(rid, err, op)
			} else {
				binary.LittleEndian.PutUint32(b[4:], uint32(ridint))
				c.conn.WriteMessage(websocket.BinaryMessage, b[:(8+len)])
			}
		case "write":
			p, _ := cmd["p"].Int64()
			len, err := c.writeBlock(cmd["path"].String(), data, p)
			if err != nil {
				c.errorResponse(rid, err, "write")
			} else {
				c.response(rid, len)
			}
		case "remove":
			err := c.v.Remove(cmd["path"].String())
			if err != nil {
				c.errorResponse(rid, err, op)
			} else {
				c.response(rid, nil)
			}
		case "files":
			files, err := c.v.ReadDir(cmd["path"].String())
			if err != nil {
				c.errorResponse(rid, err, op)
			} else {
				c.response(rid, files)
			}
		case "mkdir":
			err := c.v.Mkdir(cmd["path"].String(), 0)
			if err != nil {
				c.errorResponse(rid, err, op)
			} else {
				c.response(rid, nil)
			}
		default:
			c.errorResponse(rid, nil, "unknown operation")
		}
	}
}
