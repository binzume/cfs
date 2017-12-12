package volume

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
	Remove(path string) error
	Stat(path string) (*FileStat, error)
	Available() bool
}

type VolumeGroup struct {
	vv   []volumeGroupEntry
	lock sync.Mutex
}

type volumeGroupEntry struct {
	p string
	v Volume
}

// NewVolumeGroup returns empty volume.
func NewVolumeGroup() *VolumeGroup {
	return &VolumeGroup{}
}

func (vg *VolumeGroup) Add(path string, v Volume) {
	vg.lock.Lock()
	defer vg.lock.Unlock()
	vg.vv = append(vg.vv, volumeGroupEntry{path, v})
}

func (v *VolumeGroup) Locker() sync.Locker {
	return &v.lock
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

func (vg *VolumeGroup) Remove(path string) error {
	if v, p, ok := vg.resolve(path); ok {
		return v.Remove(p)
	}
	return fmt.Errorf("not supported %s", path)
}

func (vg *VolumeGroup) ReadDir(path string) ([]*File, error) {
	files := []*File{} // TODO uniq.

	if v, p, ok := vg.resolve(path); ok {
		files, _ = v.ReadDir(p)
	}

	if path != "" {
		path += "/"
	}
	for _, e := range vg.vv {
		if e.v.Available() && strings.HasPrefix(e.p, path) {
			n := strings.Split(e.p[len(path):], "/")[0]
			files = append(files, &File{FileStat: FileStat{IsDir: true}, Name: n})
		}
	}
	return files, nil
}

func (vg *VolumeGroup) Available() bool {
	return true
}

func (vg *VolumeGroup) resolve(path string) (Volume, string, bool) {
	for _, e := range vg.vv {
		if !e.v.Available() {
			continue
		}
		if e.p == path {
			return e.v, "", true
		}
		if strings.HasPrefix(path, e.p+"/") {
			return e.v, path[len(e.p)+1:], true
		}
	}
	return nil, "", false
}
