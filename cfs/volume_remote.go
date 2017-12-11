package main

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
	statCache map[string]*statCache
	connected bool
}

type statCache struct {
	stat *FileStat
	time time.Time
}

var _ Volume = &RemoteVolume{}
var statCacheExpireTime = time.Second * 5

func NewRemoteVolume(name string, conn *websocket.Conn) *RemoteVolume {
	return &RemoteVolume{
		Name:      name,
		conn:      conn,
		wch:       make(chan map[string]interface{}),
		statCache: make(map[string]*statCache),
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
	if s, ok := v.statCache[path]; ok {
		if s.time.Add(statCacheExpireTime).After(time.Now()) {
			return s.stat, nil
			delete(v.statCache, path)
		}
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
	v.statCache[path] = &statCache{stat: res.S, time: time.Now()}
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
	delete(v.statCache, path)
	return res["l"], nil
}

func (v *RemoteVolume) Remove(path string) error {
	var res map[string]interface{}
	delete(v.statCache, path)
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
		v.statCache[path+"/"+f.Name] = &statCache{stat: &f.FileStat, time: time.Now()}
	}
	return res.Files, nil
}

func (v *RemoteVolume) Available() bool {
	return v.connected
}
