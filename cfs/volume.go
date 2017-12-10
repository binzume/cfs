package main

import (
	"fmt"
	"strings"
	"sync"
)

type FileStat struct {
	Size        int64 `json:"size"`
	CreatedTime int64 `json:"created_time"`
	UpdatedTime int64 `json:"updated_time"`
	IsDir       bool  `json:"is_directory"`
}

type File struct {
	FileStat
	Name string `json:"name"`
}

type Volume interface {
	Locker() sync.Locker
	ReadDir(path string) ([]*File, error)
	Read(path string, b []byte, offset int64) (int, error)
	Write(path string, b []byte, offset int64) (int, error)
	Stat(path string) (*FileStat, error)
}

type VolumeGroup struct {
	vv   map[string]Volume
	lock sync.Mutex
}

func NewVolumeGroup() *VolumeGroup {
	return &VolumeGroup{vv: make(map[string]Volume)}
}

func (v *VolumeGroup) Locker() sync.Locker {
	return &v.lock
}

func (vg *VolumeGroup) Add(path string, v Volume) {
	vg.vv[path] = v
}

func (vg *VolumeGroup) Stat(path string) (*FileStat, error) {
	if v, p, ok := vg.resolve(path); ok {
		return v.Stat(p)
	}
	return &FileStat{IsDir: true}, nil
}

func (vg *VolumeGroup) Read(path string, b []byte, offset int64) (int, error) {
	if v, p, ok := vg.resolve(path); ok {
		return v.Read(p, b, offset)
	}
	return 0, fmt.Errorf("not supported %s", path)
}

func (vg *VolumeGroup) Write(path string, b []byte, offset int64) (int, error) {
	if v, p, ok := vg.resolve(path); ok {
		return v.Write(p, b, offset)
	}
	return 0, fmt.Errorf("not supported %s", path)
}

func (vg *VolumeGroup) ReadDir(path string) ([]*File, error) {
	files := []*File{}

	if v, p, ok := vg.resolve(path); ok {
		files, _ = v.ReadDir(p)
	}

	if path != "" {
		path += "/"
	}
	for p := range vg.vv {
		if strings.HasPrefix(p, path) {
			n := strings.Split(p[len(path):], "/")[0]
			files = append(files, &File{FileStat: FileStat{IsDir: true}, Name: n})
		}
	}
	return files, nil
}

func (vg *VolumeGroup) resolve(path string) (Volume, string, bool) {
	for p, v := range vg.vv {
		if p == path {
			return v, "", true
		}
		if strings.HasPrefix(path, p+"/") {
			return v, path[len(p)+1:], true
		}
	}
	return nil, "", false
}
