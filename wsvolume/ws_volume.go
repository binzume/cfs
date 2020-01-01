package wsvolume

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/binzume/cfs/volume"
)

type rmsg struct {
	messageType int
	message     []byte
}

type ReqData map[string]interface{}
type Req struct {
	data       ReqData
	responseCh chan<- rmsg
}

// WebsocketVolume ...
type WebsocketVolume struct {
	Name      string
	lock      sync.Mutex
	conn      io.Closer // TODO: multiple conns
	wch       chan Req
	statCache statCache
	connected bool
}

type statCache struct {
	c    map[string]*statCacheE
	lock sync.Mutex
}
type statCacheE struct {
	stat *volume.FileInfo
	time time.Time
}

type websocketConnector func() (*websocket.Conn, error)

// NewWebsocketVolume returns a new volume.
func NewWebsocketVolume(name string) *WebsocketVolume {
	return &WebsocketVolume{
		Name:      name,
		wch:       make(chan Req),
		statCache: statCache{c: map[string]*statCacheE{}},
	}
}

// WSUpgrader for upgrading http request in handle request
var WSUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var statCacheExpireTime = time.Second * 5

func (c *statCache) set(path string, stat *volume.FileInfo) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.c[path] = &statCacheE{stat: stat, time: time.Now()}
}
func (c *statCache) get(path string) (*volume.FileInfo, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if s, ok := c.c[path]; ok {
		if s.time.Add(statCacheExpireTime).After(time.Now()) {
			return s.stat, true
		}
		delete(c.c, path)
	}
	return nil, false
}
func (c *statCache) delete(path string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.c, path)
}

// Start volume backend.
func (v *WebsocketVolume) StartClient(connector websocketConnector) (<-chan struct{}, error) {
	conn, err := connector()
	if err != nil {
		log.Println("failed to connect: ", err)
		return nil, err
	}

	log.Println("start volume.", v.Name)

	return v.BindConnection(conn)
}

func (v *WebsocketVolume) StartClientWithDefaultConnector(wsurl string) (<-chan struct{}, error) {
	var connector = func() (*websocket.Conn, error) {
		c, _, err := websocket.DefaultDialer.Dial(wsurl, nil)
		return c, err
	}
	return v.StartClient(connector)
}

func (v *WebsocketVolume) HandleRequest(w http.ResponseWriter, r *http.Request, header http.Header) (<-chan struct{}, error) {
	conn, err := WSUpgrader.Upgrade(w, r, header)
	if err != nil {
		return nil, err
	}
	return v.BindConnection(conn)
}

func (v *WebsocketVolume) BindConnection(conn *websocket.Conn) (<-chan struct{}, error) {
	v.lock.Lock()
	defer v.lock.Unlock()

	if v.connected {
		return nil, fmt.Errorf("Already connected")
	}
	v.conn = conn
	v.connected = true

	done := make(chan struct{})
	var data = map[string]string{}
	conn.ReadJSON(data) // wait to establish.

	reqsCh := make(chan Req, 1)

	go func() {
		defer v.Terminate()
		defer close(reqsCh)
		count := 0
		for {
			select {
			case req := <-v.wch:
				req.data["rid"] = count
				err := conn.WriteJSON(req.data)
				if err != nil {
					close(req.responseCh)
					break
				}
				reqsCh <- req
				count++
			case <-done:
				break
			}
		}
	}()

	go func() {
		defer v.Terminate()
		defer close(done)
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			req := <-reqsCh
			// TODO: check rid.
			req.responseCh <- rmsg{mt, msg}
		}
		for req := range reqsCh {
			close(req.responseCh)
		}
	}()

	return done, nil
}

// Stop volume backend.
func (v *WebsocketVolume) Terminate() {
	v.lock.Lock()
	defer v.lock.Unlock()

	v.connected = false
	if v.conn != nil {
		v.conn.Close()
		v.conn = nil
	}
	log.Println("terminate volume.", v.Name)
}

func (v *WebsocketVolume) requestRaw(r ReqData) (rmsg, error) {
	if !v.connected {
		return rmsg{}, fmt.Errorf("connection closed")
	}
	// TODO: timeout
	rch := make(chan rmsg)
	v.wch <- Req{
		data:       r,
		responseCh: rch,
	}
	res := <-rch
	if res.message == nil {
		return res, fmt.Errorf("connection closed")
	}
	return res, nil
}

func (v *WebsocketVolume) request(r ReqData, result interface{}) error {
	res, err := v.requestRaw(r)
	if err != nil {
		return err
	}
	if res.messageType != websocket.TextMessage {
		return fmt.Errorf("invalid msgType")
	}
	return json.Unmarshal(res.message, &result)
}

func (v *WebsocketVolume) Available() bool {
	return v.connected
}

func (v *WebsocketVolume) Stat(path string) (*volume.FileInfo, error) {
	if s, ok := v.statCache.get(path); ok {
		if s == nil {
			return nil, volume.NoentError
		}
		stat := *s
		stat.Path = path
		return &stat, nil
	}
	var res struct {
		S   *volume.FileInfo `json:"stat"`
		Err string           `json:"error"`
	}
	err := v.request(ReqData{"op": "stat", "path": path}, &res)
	if err != nil {
		return nil, err
	}
	if res.Err == "noent" {
		v.statCache.set(path, nil)
		return nil, volume.NoentError
	}
	if res.S == nil {
		log.Printf("invalid response :%v", res.Err)
		return nil, fmt.Errorf("invalid response %v", res.Err)
	}
	v.statCache.set(path, res.S)
	return res.S, nil
}

type fileHandle struct {
	volume *WebsocketVolume
	path   string
}

func (f *fileHandle) ReadAt(b []byte, offset int64) (int, error) {
	v := f.volume
	msg, err := v.requestRaw(ReqData{"op": "read", "path": f.path, "p": offset, "l": len(b)})
	if err != nil {
		return 0, err
	}

	if msg.messageType != websocket.BinaryMessage {
		return 0, fmt.Errorf("invalid msgType")
	}
	if len(msg.message) == 0 {
		return 0, io.EOF
	}
	return copy(b, msg.message), nil
}

func (f *fileHandle) WriteAt(b []byte, offset int64) (int, error) {
	v := f.volume
	var res struct {
		Len int    `json:"l"`
		Err string `json:"error"`
	}
	err := v.request(ReqData{"op": "write", "path": f.path, "p": offset, "b": string(b)}, &res)
	if err != nil {
		return 0, err
	}
	v.statCache.delete(f.path)
	return res.Len, nil
}

type fileReadWriter struct {
	*fileHandle
	pos int64
}

func (f *fileReadWriter) Write(data []byte) (int, error) {
	len, err := f.WriteAt(data, f.pos)
	f.pos += int64(len)
	return len, err
}

func (f *fileReadWriter) Read(data []byte) (int, error) {
	len, err := f.ReadAt(data, f.pos)
	f.pos += int64(len)
	return len, err
}

func (f *fileReadWriter) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekCurrent {
		f.pos += offset
	} else if whence == io.SeekStart {
		f.pos = offset
	} else {
		return f.pos, errors.New("invalid whence")
	}
	return f.pos, nil
}

func (*fileReadWriter) Close() error {
	return nil
}

func (v *WebsocketVolume) Open(path string) (volume.FileReadCloser, error) {
	return &fileReadWriter{&fileHandle{v, path}, 0}, nil
}

func (v *WebsocketVolume) Create(path string) (volume.FileWriteCloser, error) {
	v.statCache.delete(path)
	return &fileReadWriter{&fileHandle{v, path}, 0}, nil
}

func (v *WebsocketVolume) OpenFile(path string, flag int, perm os.FileMode) (volume.File, error) {
	return &fileReadWriter{&fileHandle{v, path}, 0}, nil
}

func (v *WebsocketVolume) Remove(path string) error {
	var res struct {
		Err string `json:"error"`
	}
	v.statCache.delete(path)
	return v.request(map[string]interface{}{"op": "remove", "path": path}, &res)
}

func (v *WebsocketVolume) ReadDir(fpath string) ([]*volume.FileInfo, error) {
	var res struct {
		Files []*volume.FileInfo `json:"files"`
	}
	err := v.request(map[string]interface{}{"op": "files", "path": fpath}, &res)
	if err != nil {
		return nil, err
	}
	for _, f := range res.Files {
		v.statCache.set(path.Join(fpath, f.Path), f)
	}
	return res.Files, nil
}

func (v *WebsocketVolume) Mkdir(path string, mode os.FileMode) error {
	var res struct {
		Err string `json:"error"`
	}
	return v.request(map[string]interface{}{"op": "mkdir", "path": path}, &res)
}
