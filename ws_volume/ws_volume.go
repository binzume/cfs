package ws_volume

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/binzume/cfs/volume"
)

// RemoteVolume ...
type RemoteVolume struct {
	Name      string
	lock      sync.Mutex
	connector websocketConnector
	conn      *websocket.Conn // TODO: multiple conns
	wch       chan map[string]interface{}
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

type websocketConnector func(*RemoteVolume) (*websocket.Conn, error)

// NewRemoteVolume returns a new volume.
func NewRemoteVolume(name string, conn websocketConnector) *RemoteVolume {
	return &RemoteVolume{
		Name:      name,
		connector: conn,
		wch:       make(chan map[string]interface{}),
		statCache: statCache{c: map[string]*statCacheE{}},
	}
}

var statCacheExpireTime = time.Second * 5

func (c *statCache) set(path string, stat *volume.FileInfo) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.c[path] = &statCacheE{stat: stat, time: time.Now()}
}
func (c *statCache) get(path string) *volume.FileInfo {
	c.lock.Lock()
	defer c.lock.Unlock()
	if s, ok := c.c[path]; ok {
		if s.time.Add(statCacheExpireTime).After(time.Now()) {
			return s.stat
		}
		delete(c.c, path)
	}
	return nil
}
func (c *statCache) delete(path string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.c, path)
}

// Start volume backend.
func (v *RemoteVolume) Start() (<-chan error, error) {
	v.lock.Lock()
	defer v.lock.Unlock()
	errorch := make(chan error)

	conn, err := v.connector(v)
	if err != nil {
		log.Println("failed to connect: ", err)
		return errorch, err
	}
	v.conn = conn
	var data = map[string]string{}
	v.conn.ReadJSON(data) // wait to establish.

	log.Println("start volume.", v.Name)

	v.connected = true
	go func() {
		defer v.Terminate()
		count := 0
		for {
			req := <-v.wch
			req["rid"] = count
			err := v.conn.WriteJSON(req)
			if err != nil {
				errorch <- err
				break
			}
			count++
		}
	}()
	return errorch, nil
}

// Stop volume backend.
func (v *RemoteVolume) Terminate() {
	v.lock.Lock()
	defer v.lock.Unlock()

	v.connected = false
	if v.conn != nil {
		v.conn.Close()
		v.conn = nil
	}
	log.Println("terminate volume.", v.Name)
}

func (v *RemoteVolume) request(r map[string]interface{}, result interface{}) error {
	if !v.connected {
		return fmt.Errorf("connection closed")
	}
	v.wch <- r
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.conn.ReadJSON(&result)
}

func (v *RemoteVolume) Locker() sync.Locker {
	return &v.lock
}

func (v *RemoteVolume) Available() bool {
	return v.connected
}

func (v *RemoteVolume) Stat(path string) (*volume.FileInfo, error) {
	if s := v.statCache.get(path); s != nil {
		return s, nil
	}
	var res struct {
		S *volume.FileInfo `json:"stat"`
	}
	err := v.request(map[string]interface{}{"op": "stat", "path": path}, &res)
	if err != nil {
		return nil, err
	}
	if res.S == nil {
		return nil, fmt.Errorf("invalid response")
	}
	v.statCache.set(path, res.S)
	return res.S, nil
}

type fileHandle struct {
	volume *RemoteVolume
	path   string
}

func (f *fileHandle) ReadAt(b []byte, offset int64) (int, error) {
	v := f.volume
	v.wch <- map[string]interface{}{"op": "read", "path": f.path, "p": offset, "l": len(b)}

	v.lock.Lock()
	defer v.lock.Unlock()
	mt, msg, err := v.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	if mt != websocket.BinaryMessage {
		return 0, fmt.Errorf("invalid msgType")
	}
	return copy(b, msg), nil
}

func (f *fileHandle) WriteAt(b []byte, offset int64) (int, error) {
	v := f.volume
	var res map[string]int
	err := v.request(map[string]interface{}{"op": "write", "path": f.path, "p": offset, "b": string(b)}, &res)
	if err != nil {
		return 0, err
	}
	v.statCache.delete(f.path)
	return res["l"], nil
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

func (v *RemoteVolume) Open(path string) (volume.FileReadCloser, error) {
	return &fileReadWriter{&fileHandle{v, path}, 0}, nil
}

func (v *RemoteVolume) Create(path string) (volume.FileWriteCloser, error) {
	return &fileReadWriter{&fileHandle{v, path}, 0}, nil
}

func (v *RemoteVolume) OpenFile(path string, flag int, perm os.FileMode) (volume.File, error) {
	return &fileReadWriter{&fileHandle{v, path}, 0}, nil
}

func (v *RemoteVolume) Remove(path string) error {
	var res map[string]interface{}
	v.statCache.delete(path)
	return v.request(map[string]interface{}{"op": "remove", "path": path}, &res)
}

func (v *RemoteVolume) ReadDir(path string) ([]*volume.FileInfo, error) {
	var res struct {
		Files []*volume.FileInfo `json:"files"`
	}
	err := v.request(map[string]interface{}{"op": "files", "path": path}, &res)
	if err != nil {
		return nil, err
	}
	for _, f := range res.Files {
		v.statCache.set(path+"/"+f.Path, f)
	}
	return res.Files, nil
}

func (v *RemoteVolume) Mkdir(path string, mode os.FileMode) error {
	var res map[string]interface{}
	return v.request(map[string]interface{}{"op": "mkdir", "path": path}, &res)
}
