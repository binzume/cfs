package volume

import (
	"io"
	"os"
	"path"
)

type volumeWrapper struct {
	Volume
	writable bool
}

func ToFS(v Volume) FS {
	if fs, ok := v.(FS); ok {
		return fs
	}
	_, writable := v.(VolumeWriter)
	return &volumeWrapper{Volume: v, writable: writable}
}

func (fs *volumeWrapper) Walk(callback func(*FileInfo)) error {
	return walk(fs.Volume, callback)
}

func (fs *volumeWrapper) Watch(callback func(FileEvent)) (io.Closer, error) {
	return watch(fs.Volume, callback)
}

func (v *volumeWrapper) Create(path string) (FileWriteCloser, error) {
	if !v.writable {
		return nil, permissionError("Create", path)
	}
	if w, ok := v.Volume.(VolumeWriter); ok {
		return w.Create(path)
	}
	return nil, unsupportedError("Create", path)
}

func (v *volumeWrapper) Remove(path string) error {
	if !v.writable {
		return permissionError("Remove", path)
	}
	if w, ok := v.Volume.(VolumeWriter); ok {
		return w.Remove(path)
	}
	return unsupportedError("Remove", path)
}
func (v *volumeWrapper) Mkdir(path string, perm os.FileMode) error {
	if !v.writable {
		return permissionError("Mkdir", path)
	}
	if w, ok := v.Volume.(VolumeWriter); ok {
		return w.Mkdir(path, perm)
	}
	return unsupportedError("Mkdir", path)
}

func (v *volumeWrapper) OpenFile(path string, flag int, perm os.FileMode) (File, error) {
	if !v.writable {
		return nil, permissionError("OpenFile", path)
	}
	if w, ok := v.Volume.(VolumeWriter); ok {
		return w.OpenFile(path, flag, perm)
	}
	return nil, unsupportedError("OpenFile", path)
}

func (fs *volumeWrapper) WalkCh() <-chan *FileInfo {
	fch := make(chan *FileInfo)
	go func() {
		defer close(fch)
		fs.Walk(func(f *FileInfo) {
			fch <- f
		})
	}()
	return fch
}

func walk(v Volume, callback func(*FileInfo)) error {
	if w, ok := v.(VolumeWalker); ok {
		return w.Walk(callback)
	}
	return walkDir(v, callback, "")
}

func walkDir(v Volume, callback func(*FileInfo), p string) error {
	files, err := v.ReadDir(p)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() {
			err = walkDir(v, callback, f.Path)
			if err != nil {
				return err
			}
		} else {
			f.Path = path.Join(p, f.Path)
			callback(f)
		}
	}
	return nil
}

func watch(v Volume, callback func(FileEvent)) (io.Closer, error) {
	if w, ok := v.(VolumeWatcher); ok {
		return w.Watch(callback)
	}
	return nil, unsupportedError("watch", "")
}
