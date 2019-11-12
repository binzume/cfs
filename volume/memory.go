package volume

import (
	"bytes"
	"errors"
	"sync"
	"time"
)

type OnMemoryVolume struct {
	lock  sync.RWMutex
	files map[string][]byte
}

func NewOnMemoryVolume(init map[string][]byte) *OnMemoryVolume {
	if init == nil {
		init = make(map[string][]byte)
	}
	return &OnMemoryVolume{files: init}
}

func (v *OnMemoryVolume) Locker() sync.Locker {
	return &v.lock
}

func (v *OnMemoryVolume) Available() bool {
	return true
}

func (v *OnMemoryVolume) Stat(path string) (*FileStat, error) {
	if path == "" {
		return &FileStat{IsDir: true, Size: 0, UpdatedTime: time.Time{}}, nil
	}
	data := v.get(path)
	if data == nil {
		return nil, errors.New("noent")
	}
	return &FileStat{IsDir: false, Size: int64(len(data)), UpdatedTime: time.Time{}}, nil
}

func (v *OnMemoryVolume) Remove(path string) error {
	v.lock.Lock()
	defer v.lock.Unlock()
	delete(v.files, path)
	return nil
}

func (v *OnMemoryVolume) ReadDir(path string) ([]*FileEntry, error) {
	v.lock.RLock()
	defer v.lock.RUnlock()
	files := []*FileEntry{}
	for name := range v.files {
		f, err := v.Stat(name)
		if err == nil {
			files = append(files, &FileEntry{FileStat: *f, Path: name})
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
		return nil, errors.New("noent")
	}
	return &MemReadCloser{bytes.NewReader(data)}, nil
}

func (v *OnMemoryVolume) get(path string) []byte {
	v.lock.RLock()
	defer v.lock.RUnlock()
	return v.files[path]
}

func (v *OnMemoryVolume) Walk(callback func(*FileEntry)) error {
	for name, data := range v.files {
		callback(&FileEntry{
			FileStat: FileStat{
				IsDir:       false,
				Size:        int64(len(data)),
				UpdatedTime: time.Time{},
			},
			Path: name,
		})
	}
	return nil
}
