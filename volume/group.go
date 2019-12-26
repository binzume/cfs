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
	vg.vv = append(vg.vv, volumeGroupEntry{path, v})
}

func (vg *VolumeGroup) Clear() {
	vg.lock.Lock()
	defer vg.lock.Unlock()
	vg.vv = nil
}

func (vg *VolumeGroup) Stat(path string) (*FileInfo, error) {
	if v, p, ok := vg.resolve(path); ok {
		return v.Stat(p)
	}
	return &FileInfo{Path: path, IsDirectory: true}, nil
}

func (vg *VolumeGroup) Remove(path string) error {
	if v, p, ok := vg.resolve(path); ok {
		if vw, ok := v.(VolumeWriter); ok {
			return vw.Remove(p)
		}
	}
	return unsupportedError("Remove", path)
}

func (vg *VolumeGroup) Mkdir(path string, perm os.FileMode) error {
	if v, p, ok := vg.resolve(path); ok {
		if vw, ok := v.(VolumeWriter); ok {
			return vw.Mkdir(p, perm)
		}
	}
	return unsupportedError("Mkdir", path)
}

func (vg *VolumeGroup) Create(path string) (FileWriteCloser, error) {
	if v, p, ok := vg.resolve(path); ok {
		if vw, ok := v.(VolumeWriter); ok {
			return vw.Create(p)
		}
	}
	return nil, unsupportedError("Create", path)
}

func (vg *VolumeGroup) OpenFile(path string, flag int, perm os.FileMode) (File, error) {
	if v, p, ok := vg.resolve(path); ok {
		if vw, ok := v.(VolumeWriter); ok {
			return vw.OpenFile(p, flag, perm)
		}
		if flag == 0 {
			// readonly
			f, err := v.Open(p)
			if err != nil {
				return nil, err
			}
			return &struct {
				FileReadCloser
				io.WriterAt
				io.Writer
			}{f, nil, nil}, nil
		}
	}
	return nil, unsupportedError("OpenFile", path)
}

func (vg *VolumeGroup) ReadDir(path string) ([]*FileInfo, error) {
	files := []*FileInfo{} // TODO uniq.

	if v, p, ok := vg.resolve(path); ok {
		files, _ = v.ReadDir(p)
	}

	if path != "" {
		path += "/"
	}
	vg.lock.RLock()
	defer vg.lock.RUnlock()
	for _, e := range vg.vv {
		if e.v.Available() && strings.HasPrefix(e.p, path) {
			n := strings.Split(e.p[len(path):], "/")[0]
			files = append(files, &FileInfo{IsDirectory: true, Path: n})
		}
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
		e := closer.Close()
		if e != nil {
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
				f.File.Path = path + "/" + f.File.Path
				callback(f)
			})
			if c != nil {
				closers = append(closers, c)
			}
		}
	}
	return &multiCloser{closers}, nil
}

func (vg *VolumeGroup) resolve(path string) (Volume, string, bool) {
	vg.lock.RLock()
	defer vg.lock.RUnlock()
	for _, e := range vg.vv {
		if !e.v.Available() {
			continue
		}
		if e.p == path {
			return e.v, "", true
		}
		if e.p == "" {
			return e.v, path, true
		}
		if strings.HasPrefix(path, e.p+"/") {
			return e.v, path[len(e.p)+1:], true
		}
	}
	return nil, "", false
}
