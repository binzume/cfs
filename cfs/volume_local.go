package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

// LocalVolume ...
type LocalVolume struct {
	lock      sync.Mutex
	LocalPath string
	Name      string
	writable  bool
	scan      bool
}

var _ Volume = &LocalVolume{}

// LocalFile ...
type LocalFile struct {
	parent    *Volume
	LocalPath string
	Path      string
	Updated   int64
}

func NewLocalVolume(path string, name string, writable bool) *LocalVolume {
	return &LocalVolume{LocalPath: path, Name: name, writable: writable}
}

func (v *LocalVolume) localPath(path string) string {
	return v.LocalPath + "/" + path
}

func (v *LocalVolume) Locker() sync.Locker {
	return &v.lock
}

func (v *LocalVolume) Stat(path string) (*FileStat, error) {
	fi, err := os.Stat(v.localPath(path))
	if err != nil {
		return nil, err
	}
	return &FileStat{IsDir: fi.IsDir(), Size: fi.Size(), UpdatedTime: fi.ModTime().UnixNano()}, nil
}

func (v *LocalVolume) Read(path string, b []byte, offset int64) (int, error) {
	file, err := os.Open(v.localPath(path))
	if err != nil {
		return 0, err
	}
	defer file.Close()
	return file.ReadAt(b, offset)
}

func (v *LocalVolume) Write(path string, b []byte, offset int64) (int, error) {
	if !v.writable {
		return 0, fmt.Errorf("readonly %s", path)
	}
	return 0, fmt.Errorf("not supported %s", path)
}

func (v *LocalVolume) ReadDir(path string) ([]*File, error) {
	items, err := ioutil.ReadDir(v.localPath(path))
	if err != nil {
		return nil, err
	}
	files := []*File{}

	for _, fi := range items {
		f := &File{
			FileStat: FileStat{IsDir: fi.IsDir(), Size: fi.Size(),
				UpdatedTime: fi.ModTime().UnixNano()},
			Name: fi.Name(),
		}
		files = append(files, f)
	}

	return files, nil
}
