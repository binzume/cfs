package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// RemoteVolume ...
type RemoteVolume struct {
	Name string
	lock sync.Mutex
	conn *websocket.Conn // TODO: multiple conns
	wch  chan map[string]interface{}
}

var _ Volume = &RemoteVolume{}

func NewRemoteVolume(name string, conn *websocket.Conn) *RemoteVolume {
	return &RemoteVolume{Name: name, conn: conn, wch: make(chan map[string]interface{})}
}

func (v *RemoteVolume) Start() {
	go func() {
		for {
			req := <-v.wch
			err := v.conn.WriteJSON(req)
			if err != nil {
				break
			}
		}
	}()
	log.Println("terminate volume.")
}

func (v *RemoteVolume) request(r map[string]interface{}, result interface{}) error {
	v.wch <- r
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.conn.ReadJSON(&result)
}

func (v *RemoteVolume) Locker() sync.Locker {
	return &v.lock
}

func (v *RemoteVolume) Stat(path string) (*FileStat, error) {
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
	return res["l"], nil
}

func (v *RemoteVolume) Remove(path string) error {
	var res map[string]interface{}
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
	return res.Files, nil
}
