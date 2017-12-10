package main

import (
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

// RemoteVolume ...
type RemoteVolume struct {
	Name string
	lock sync.Mutex
	conn *websocket.Conn // TODO: multiple conns
}

var _ Volume = &RemoteVolume{}

func NewRemoteVolume(name string, conn *websocket.Conn) *RemoteVolume {
	return &RemoteVolume{Name: name, conn: conn}
}

func (v *RemoteVolume) Locker() sync.Locker {
	return &v.lock
}

func (v *RemoteVolume) Stat(path string) (*FileStat, error) {
	v.lock.Lock()
	defer v.lock.Unlock()
	err := v.conn.WriteJSON(map[string]string{"op": "stat", "path": path})
	if err != nil {
		return nil, err
	}
	var res struct {
		S *FileStat `json:"stat"`
	}
	err = v.conn.ReadJSON(&res)
	if err != nil {
		return nil, err
	}
	if res.S == nil {
		return nil, fmt.Errorf("invalid response")
	}
	return res.S, nil
}

func (v *RemoteVolume) Read(path string, b []byte, offset int64) (int, error) {
	v.lock.Lock()
	defer v.lock.Unlock()
	err := v.conn.WriteJSON(map[string]interface{}{"op": "read", "path": path, "p": offset, "l": len(b)})
	if err != nil {
		return 0, err
	}
	mt, msg, err := v.conn.ReadMessage()
	if err != nil {
		return 0, err
	}
	if mt != 1 {
		return 0, fmt.Errorf("invalid msgType")
	}
	return copy(b, msg), nil
}

func (v *RemoteVolume) Write(path string, b []byte, offset int64) (int, error) {
	return 0, fmt.Errorf("not implemented %s", path)
}

func (v *RemoteVolume) ReadDir(path string) ([]*File, error) {
	v.lock.Lock()
	defer v.lock.Unlock()
	err := v.conn.WriteJSON(map[string]string{"op": "files", "path": path})
	if err != nil {
		return nil, err
	}
	var res struct {
		Files []*File `json:"files"`
	}
	err = v.conn.ReadJSON(&res)
	if err != nil {
		return nil, err
	}
	return res.Files, nil
}
