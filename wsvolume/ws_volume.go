package wsvolume

import (
	"encoding/binary"
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
	data   []byte
	binary bool
	err    error
}

type ReqData map[string]interface{}
type Cmd struct {
	req   ReqData
	resCh chan<- *rmsg
}

type ResponseJson struct {
	RID   uint32          `json:"rid"`
	Error *string         `json:"error"`
	Data  json.RawMessage `json:"data"`
}

type RemoteError string

func (e *RemoteError) Error() string {
	return string(*e)
}

// WebsocketVolume ...
type WebsocketVolume struct {
	Name      string
	lock      sync.Mutex
	conn      io.Closer // TODO: multiple conns
	wch       chan Cmd
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
		wch:       make(chan Cmd),
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

func (v *WebsocketVolume) readResponse(conn *websocket.Conn) (*rmsg, uint32, error) {
	mt, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, 0, err
	}
	switch mt {
	case websocket.TextMessage:
		var res ResponseJson
		if err := json.Unmarshal(msg, &res); err != nil {
			return nil, 0, err
		}
		if res.Error != nil {
			err = (*RemoteError)(res.Error)
		}
		return &rmsg{res.Data, false, err}, res.RID, nil
	case websocket.BinaryMessage:
		if len(msg) < 8 {
			return nil, 0, fmt.Errorf("invalid binary response")
		}
		rid := binary.LittleEndian.Uint32(msg[4:])
		return &rmsg{msg[8:], true, nil}, rid, nil
	default:
		return nil, 0, fmt.Errorf("invalid message type")
	}
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

	cmds := map[uint32]Cmd{}
	var cmdsLock sync.Mutex

	go func() {
		defer v.Terminate()
		var ridSeq uint32
		defer func() {
			// cancel commands
			cmdsLock.Lock()
			for _, cmd := range cmds {
				close(cmd.resCh)
			}
			cmdsLock.Unlock()
		}()
		for {
			select {
			case cmd := <-v.wch:
				ridSeq++
				cmd.req["rid"] = ridSeq
				err := conn.WriteJSON(cmd.req)
				if err != nil {
					close(cmd.resCh)
					return
				}
				cmdsLock.Lock()
				cmds[ridSeq] = cmd
				cmdsLock.Unlock()
			case <-done:
				return
			}
		}
	}()

	go func() {
		defer v.Terminate()
		defer close(done)
		for {
			r, rid, err := v.readResponse(conn)
			if err != nil {
				return
			}
			cmdsLock.Lock()
			if cmd, ok := cmds[rid]; ok {
				delete(cmds, rid)
				cmd.resCh <- r
			}
			cmdsLock.Unlock()
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

func (v *WebsocketVolume) requestRaw(r ReqData) (*rmsg, error) {
	if !v.connected {
		return nil, fmt.Errorf("connection closed")
	}
	rch := make(chan *rmsg, 1)
	v.wch <- Cmd{
		req:   r,
		resCh: rch,
	}
	select {
	case res := <-rch:
		if res == nil {
			return nil, fmt.Errorf("connection closed")
		}
		return res, res.err
	case <-time.After(15 * time.Second):
	}
	return nil, fmt.Errorf("timeout")
}

func (v *WebsocketVolume) request(r ReqData, result interface{}) error {
	rmsg, err := v.requestRaw(r)
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(rmsg.data, result)
	}
	return nil
}

func (v *WebsocketVolume) Available() bool {
	return v.connected
}

func (v *WebsocketVolume) Stat(path string) (*volume.FileInfo, error) {
	if s, ok := v.statCache.get(path); ok {
		if s == nil {
			return nil, &os.PathError{
				Op:   "stat",
				Path: path,
				Err:  volume.NoentError,
			}
		}
		stat := *s
		stat.Path = path
		return &stat, nil
	}
	var stat volume.FileInfo
	err := v.request(ReqData{"op": "stat", "path": path}, &stat)
	if err != nil {
		if err.Error() == "noent" {
			v.statCache.set(path, nil)
			return nil, &os.PathError{
				Op:   "stat",
				Path: path,
				Err:  volume.NoentError,
			}
		}
		return nil, err
	}
	if stat.Path == "" {
		return nil, fmt.Errorf("stat: invalid response")
	}
	v.statCache.set(path, &stat)
	return &stat, nil
}

type fileHandle struct {
	volume           *WebsocketVolume
	path             string
	lastReadPos      int64
	seqReadCount     int
	readBufferOffset int64
	readBuffer       []byte
}

func (f *fileHandle) ReadAt(b []byte, offset int64) (int, error) {
	if offset >= f.readBufferOffset && offset+int64(len(b)) <= f.readBufferOffset+int64(len(f.readBuffer)) {
		return copy(b, f.readBuffer[offset-f.readBufferOffset:]), nil
	}

	sz := len(b)
	if offset == f.lastReadPos {
		f.seqReadCount++
		if f.seqReadCount > 2 && sz < 32768 && sz > 0 {
			sz *= (32768 / sz)
		}
	}
	v := f.volume
	msg, err := v.requestRaw(ReqData{"op": "read", "path": f.path, "p": offset, "l": sz})
	if err != nil {
		return 0, err
	}
	if !msg.binary {
		return 0, fmt.Errorf("invalid msgType")
	}
	if len(msg.data) == 0 {
		return 0, io.EOF
	}
	f.lastReadPos = offset + int64(len(msg.data))
	f.readBufferOffset = offset
	f.readBuffer = msg.data
	return copy(b, msg.data), nil
}

func (f *fileHandle) WriteAt(b []byte, offset int64) (int, error) {
	f.seqReadCount = 0
	f.readBuffer = nil
	v := f.volume
	var len int
	err := v.request(ReqData{"op": "write", "path": f.path, "p": offset, "b": string(b)}, &len)
	if err != nil {
		return 0, err
	}
	v.statCache.delete(f.path)
	return len, nil
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
	return &fileReadWriter{&fileHandle{volume: v, path: path}, 0}, nil
}

func (v *WebsocketVolume) Create(path string) (volume.FileWriteCloser, error) {
	v.statCache.delete(path)
	return &fileReadWriter{&fileHandle{volume: v, path: path}, 0}, nil
}

func (v *WebsocketVolume) OpenFile(path string, flag int, perm os.FileMode) (volume.File, error) {
	return &fileReadWriter{&fileHandle{volume: v, path: path}, 0}, nil
}

func (v *WebsocketVolume) Remove(path string) error {
	v.statCache.delete(path)
	return v.request(map[string]interface{}{"op": "remove", "path": path}, nil)
}

func (v *WebsocketVolume) ReadDir(fpath string) ([]*volume.FileInfo, error) {
	files := []*volume.FileInfo{}
	err := v.request(map[string]interface{}{"op": "files", "path": fpath}, &files)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		v.statCache.set(path.Join(fpath, f.Path), f)
	}
	return files, nil
}

func (v *WebsocketVolume) Mkdir(path string, mode os.FileMode) error {
	return v.request(map[string]interface{}{"op": "mkdir", "path": path}, nil)
}
