package wsvolume

import (
	"bytes"
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
	typ    int
	data   []byte
	binary bool
	err    error
}

type ReqData map[string]interface{}
type Cmd struct {
	req      ReqData
	bindata  []byte
	resCh    chan<- *rmsg
	canceled bool
}

const (
	MessageTypeResponce = 0
	MessageTypeNotify   = 2
)

type ResponseJson struct {
	Type  int             `json:"type"`
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
	Name           string
	lock           sync.Mutex
	conn           io.Closer // TODO: multiple conns
	wch            chan *Cmd
	statCache      statCache
	notifyCallback func(data []byte)
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
		wch:       make(chan *Cmd),
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
func (c *statCache) checkAll() {
	now := time.Now()
	c.lock.Lock()
	defer c.lock.Unlock()
	for path, s := range c.c {
		if now.After(s.time.Add(statCacheExpireTime)) {
			delete(c.c, path)
		}
	}
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

type wsVolumeConn struct {
	conn     *websocket.Conn
	cmds     map[uint32]*Cmd
	cmdsLock sync.Mutex
	ridSeq   uint32
}

func (c *wsVolumeConn) sendCommand(cmd *Cmd) error {
	c.ridSeq++
	cmd.req["rid"] = c.ridSeq
	if cmd.bindata == nil {
		err := c.conn.WriteJSON(cmd.req)
		if err != nil {
			return err
		}
	} else {
		buf := new(bytes.Buffer)
		j, _ := json.Marshal(cmd.req)
		binary.Write(buf, binary.LittleEndian, uint32(0))
		binary.Write(buf, binary.LittleEndian, uint32(len(j)))
		buf.Write(j)
		buf.Write(cmd.bindata)
		err := c.conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
		if err != nil {
			return err
		}
	}
	c.cmdsLock.Lock()
	c.cmds[c.ridSeq] = cmd
	c.cmdsLock.Unlock()
	return nil
}

func (c *wsVolumeConn) readMessage() (*rmsg, uint32, error) {
	mt, msg, err := c.conn.ReadMessage()
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
		return &rmsg{res.Type, res.Data, false, err}, res.RID, nil
	case websocket.BinaryMessage:
		if len(msg) < 8 {
			return nil, 0, fmt.Errorf("invalid binary response")
		}
		typ := binary.LittleEndian.Uint32(msg[0:])
		rid := binary.LittleEndian.Uint32(msg[4:])
		return &rmsg{int(typ), msg[8:], true, nil}, rid, nil
	default:
		return nil, 0, fmt.Errorf("invalid message type")
	}
}

func (c *wsVolumeConn) setResult(rid uint32, result *rmsg) {
	c.cmdsLock.Lock()
	defer c.cmdsLock.Unlock()
	if cmd, ok := c.cmds[rid]; ok {
		delete(c.cmds, rid)
		if !cmd.canceled {
			cmd.resCh <- result
		}
		close(cmd.resCh)
	}
}

func (c *wsVolumeConn) Close() error {
	// cancel commands
	c.cmdsLock.Lock()
	for _, cmd := range c.cmds {
		close(cmd.resCh)
	}
	c.cmdsLock.Unlock()
	return c.conn.Close()
}

func (v *WebsocketVolume) BindConnection(conn *websocket.Conn) (<-chan struct{}, error) {
	v.lock.Lock()
	defer v.lock.Unlock()

	if v.conn != nil {
		return nil, fmt.Errorf("Already connected")
	}

	c := &wsVolumeConn{conn: conn, cmds: map[uint32]*Cmd{}}

	v.conn = c
	done := make(chan struct{})
	var data = map[string]string{}
	conn.ReadJSON(data) // wait to establish.

	go func() {
		defer v.Terminate()
		for {
			select {
			case cmd := <-v.wch:
				if err := c.sendCommand(cmd); err != nil {
					close(cmd.resCh)
				}
			case <-done:
				return
			}
		}
	}()

	go func() {
		defer v.Terminate()
		defer close(done)
		for {
			msg, rid, err := c.readMessage()
			if err != nil {
				return
			}
			if msg.typ == MessageTypeResponce {
				c.setResult(rid, msg)
			} else if msg.typ == MessageTypeNotify {
				if v.notifyCallback != nil {
					v.notifyCallback(msg.data)
				}
			}
		}
	}()

	return done, nil
}

// Stop volume backend.
func (v *WebsocketVolume) Terminate() {
	v.lock.Lock()
	defer v.lock.Unlock()

	if v.conn != nil {
		v.conn.Close()
		v.conn = nil
	}
	log.Println("terminate volume.", v.Name)
}

func (v *WebsocketVolume) requestRaw(r ReqData, bindata []byte) (*rmsg, error) {
	if v.conn == nil {
		return nil, fmt.Errorf("connection closed")
	}
	rch := make(chan *rmsg, 1)
	cmd := &Cmd{
		req:     r,
		bindata: bindata,
		resCh:   rch,
	}
	v.wch <- cmd
	select {
	case res := <-rch:
		if res == nil {
			return nil, fmt.Errorf("connection closed")
		}
		return res, res.err
	case <-time.After(15 * time.Second):
		cmd.canceled = true
	}
	return nil, fmt.Errorf("timeout")
}

func (v *WebsocketVolume) request(r ReqData, result interface{}) error {
	rmsg, err := v.requestRaw(r, nil)
	if err != nil {
		if err.Error() == "noent" {
			path := r["path"].(string)
			op := r["op"].(string)
			v.statCache.set(path, nil)
			return &os.PathError{
				Op:   op,
				Path: path,
				Err:  volume.NoentError,
			}
		}
		return err
	}
	if result != nil {
		return json.Unmarshal(rmsg.data, result)
	}
	return nil
}

func (v *WebsocketVolume) Available() bool {
	return v.conn != nil
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
		return nil, err
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
	// if offset >= f.readBufferOffset && offset+int64(len(b)) <= f.readBufferOffset+int64(len(f.readBuffer)) {
	if offset >= f.readBufferOffset && offset < f.readBufferOffset+int64(len(f.readBuffer)) {
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
	msg, err := v.requestRaw(ReqData{"op": "read", "path": f.path, "p": offset, "l": sz}, nil)
	if err != nil {
		return 0, err
	}
	if !msg.binary {
		return 0, fmt.Errorf("invalid msgType")
	}
	if len(msg.data) == 0 {
		return 0, io.EOF
	}
	l := copy(b, msg.data)
	f.lastReadPos = offset + int64(l)
	f.readBufferOffset = offset
	f.readBuffer = msg.data
	return l, nil
}

func (f *fileHandle) WriteAt(b []byte, offset int64) (int, error) {
	f.seqReadCount = 0
	f.readBuffer = nil
	v := f.volume
	var len int

	_, err := v.requestRaw(ReqData{"op": "write", "path": f.path, "p": offset}, b)
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
