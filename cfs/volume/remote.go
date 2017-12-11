package volume

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RemoteVolume ...
type RemoteVolume struct {
	Name      string
	lock      sync.Mutex
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
	stat *FileStat
	time time.Time
}

func (c *statCache) set(path string, stat *FileStat) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.c[path] = &statCacheE{stat: stat, time: time.Now()}
}
func (c *statCache) get(path string) *FileStat {
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

var _ Volume = &RemoteVolume{}
var statCacheExpireTime = time.Second * 5

func NewRemoteVolume(name string, conn *websocket.Conn) *RemoteVolume {
	return &RemoteVolume{
		Name:      name,
		conn:      conn,
		wch:       make(chan map[string]interface{}),
		statCache: statCache{c: map[string]*statCacheE{}},
	}
}

func (v *RemoteVolume) Start() {
	var data = map[string]string{}
	v.conn.ReadJSON(data) // wait to establish.

	log.Println("start volume.", v.Name)

	v.connected = true
	go func() {
		defer v.Terminate()
		for {
			req := <-v.wch
			err := v.conn.WriteJSON(req)
			if err != nil {
				break
			}
		}
	}()
}
func (v *RemoteVolume) Terminate() {
	v.connected = false
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

func (v *RemoteVolume) Stat(path string) (*FileStat, error) {
	if s := v.statCache.get(path); s != nil {
		return s, nil
	}
	var res struct {
		S *FileStat `json:"stat"`
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

func (v *RemoteVolume) Read(path string, b []byte, offset int64) (int, error) {
	v.wch <- map[string]interface{}{"op": "read", "path": path, "p": offset, "l": len(b)}

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

func (v *RemoteVolume) Write(path string, b []byte, offset int64) (int, error) {
	var res map[string]int
	err := v.request(map[string]interface{}{"op": "write", "path": path, "p": offset, "b": string(b)}, &res)
	if err != nil {
		return 0, err
	}
	v.statCache.delete(path)
	return res["l"], nil
}

func (v *RemoteVolume) Remove(path string) error {
	var res map[string]interface{}
	v.statCache.delete(path)
	return v.request(map[string]interface{}{"op": "remove", "path": path}, &res)
}

func (v *RemoteVolume) ReadDir(path string) ([]*File, error) {
	var res struct {
		Files []*File `json:"files"`
	}
	err := v.request(map[string]interface{}{"op": "files", "path": path}, &res)
	if err != nil {
		return nil, err
	}
	for _, f := range res.Files {
		v.statCache.set(path+"/"+f.Name, &f.FileStat)
	}
	return res.Files, nil
}

func (v *RemoteVolume) Available() bool {
	return v.connected
}
