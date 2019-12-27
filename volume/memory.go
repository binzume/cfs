package volume

import (
	"bytes"
	"os"
	"sync"
	"time"
)

type OnMemoryVolume struct {
	lock  sync.RWMutex
	files map[string][]byte
}

func NewOnMemoryVolume(init map[string][]byte) *OnMemoryVolume {
	return &OnMemoryVolume{files: init}
}

func (v *OnMemoryVolume) Available() bool {
	return true
}

func (v *OnMemoryVolume) Stat(path string) (*FileInfo, error) {
	if path == "" {
		return &FileInfo{Path: path, FileMode: os.ModeDir, FileSize: 0, UpdatedTime: time.Time{}}, nil
	}
	data := v.get(path)
	if data == nil {
		return nil, noentError("Stat", path)
	}
	return &FileInfo{Path: path, FileSize: int64(len(data)), UpdatedTime: time.Time{}}, nil
}

func (v *OnMemoryVolume) Remove(path string) error {
	v.lock.Lock()
	defer v.lock.Unlock()
	delete(v.files, path)
	return nil
}

func (v *OnMemoryVolume) ReadDir(path string) ([]*FileInfo, error) {
	v.lock.RLock()
	defer v.lock.RUnlock()
	files := []*FileInfo{}
	for name := range v.files {
		f, err := v.Stat(name)
		if err == nil {
			files = append(files, f)
		}
	}
	return files, nil
}

type MemReadCloser struct {
	*bytes.Reader
}

func (*MemReadCloser) Close() error {
	return nil
}

func (v *OnMemoryVolume) Open(path string) (reader FileReadCloser, err error) {
	data := v.get(path)
	if data == nil {
		return nil, noentError("Open", path)
	}
	return &MemReadCloser{bytes.NewReader(data)}, nil
}

func (v *OnMemoryVolume) get(path string) []byte {
	v.lock.RLock()
	defer v.lock.RUnlock()
	return v.files[path]
}
