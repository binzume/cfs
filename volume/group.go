package volume

import (
	"io"
	"os"
	"strings"
	"sync"
)

type VolumeGroup struct {
	vv   []volumeGroupEntry
	lock sync.RWMutex
}

type volumeGroupEntry struct {
	p string
	v Volume
}

// NewVolumeGroup returns empty volume group.
func NewVolumeGroup() *VolumeGroup {
	return &VolumeGroup{}
}

func (vg *VolumeGroup) AddVolume(path string, v Volume) {
	vg.lock.Lock()
	defer vg.lock.Unlock()
	vg.vv = append(vg.vv, volumeGroupEntry{strings.TrimPrefix(path, "/"), v})
}

func (vg *VolumeGroup) Clear() {
	vg.lock.Lock()
	defer vg.lock.Unlock()
	vg.vv = nil
}

func (vg *VolumeGroup) Stat(path string) (*FileInfo, error) {
	if v, p, ok := vg.resolve(path); ok {
		stat, err := v.Stat(p)
		if stat != nil {
			stat.Path = path
		}
		return stat, err
	}

	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	if path != "" {
		path += "/"
	}
	vg.lock.RLock()
	defer vg.lock.RUnlock()
	for _, e := range vg.vv {
		if e.v.Available() && strings.HasPrefix(e.p, path) {
			return &FileInfo{FileMode: os.ModeDir, Path: path}, nil
		}
	}

	return nil, noentError("Stat", path)
}

func (vg *VolumeGroup) Remove(path string) error {
	if v, p, ok := vg.resolve(path); ok {
		return v.Remove(p)
	}
	return noentError("Remove", path)
}

func (vg *VolumeGroup) Mkdir(path string, perm os.FileMode) error {
	if v, p, ok := vg.resolve(path); ok {
		return v.Mkdir(p, perm)
	}
	return noentError("Mkdir", path)
}

func (vg *VolumeGroup) Create(path string) (FileWriteCloser, error) {
	if v, p, ok := vg.resolve(path); ok {
		return v.Create(p)
	}
	return nil, noentError("Create", path)
}

func (vg *VolumeGroup) OpenFile(path string, flag int, perm os.FileMode) (File, error) {
	if v, p, ok := vg.resolve(path); ok {
		return v.OpenFile(p, flag, perm)
	}
	return nil, noentError("OpenFile", path)
}

func (vg *VolumeGroup) ReadDir(path string) ([]*FileInfo, error) {
	files := []*FileInfo{} // TODO uniq.

	resolved := false
	if v, p, ok := vg.resolve(path); ok {
		ff, err := v.ReadDir(p)
		if err == nil {
			resolved = true
			files = ff
		}
	}

	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	if path != "" {
		path += "/"
	}
	vg.lock.RLock()
	defer vg.lock.RUnlock()
	for _, e := range vg.vv {
		if e.v.Available() && strings.HasPrefix(e.p, path) {
			n := strings.Split(e.p[len(path):], "/")[0]
			files = append(files, &FileInfo{FileMode: os.ModeDir, Path: n})
		}
	}
	if !resolved && len(files) == 0 {
		return nil, noentError("ReadDir", path)
	}
	return files, nil
}

func (vg *VolumeGroup) Open(path string) (FileReadCloser, error) {
	if v, p, ok := vg.resolve(path); ok {
		return v.Open(p)
	}
	return nil, noentError("Open", path)
}

func (vg *VolumeGroup) Available() bool {
	return true
}

func (vg *VolumeGroup) Walk(callback func(f *FileInfo)) error {
	vg.lock.RLock()
	defer vg.lock.RUnlock()
	for _, e := range vg.vv {
		if e.v.Available() {
			err := walk(e.v, func(f *FileInfo) {
				f.Path = e.p + "/" + f.Path
				callback(f)
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type multiCloser struct {
	closers []io.Closer
}

func (c *multiCloser) Close() (err error) {
	for _, closer := range c.closers {
		if e := closer.Close(); e != nil {
			err = e
		}
	}
	return
}

func (vg *VolumeGroup) Watch(callback func(f FileEvent)) (io.Closer, error) {
	var closers []io.Closer
	for _, e := range vg.vv {
		if e.v.Available() {
			path := e.p
			c, _ := watch(e.v, func(f FileEvent) {
				f.Path = path + "/" + f.Path
				callback(f)
			})
			if c != nil {
				closers = append(closers, c)
			}
		}
	}
	return &multiCloser{closers}, nil
}

func (vg *VolumeGroup) resolve(path string) (FS, string, bool) {
	path = strings.TrimPrefix(path, "/")
	vg.lock.RLock()
	defer vg.lock.RUnlock()
	for _, e := range vg.vv {
		if !e.v.Available() {
			continue
		}
		if e.p == "" || e.p == path {
			return ToFS(e.v), path[len(e.p):], true
		}
		if strings.HasPrefix(path, e.p+"/") {
			return ToFS(e.v), path[len(e.p)+1:], true
		}
	}
	return nil, "", false
}
