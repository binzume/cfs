package volume

import (
	"fmt"
	"io"
	"os"
)

type volumeWrapper struct {
	Volume
	writable bool
}

func (fs *volumeWrapper) Walk(callback func(*FileInfo)) error {
	return walk(fs.Volume, callback)
}

func (fs *volumeWrapper) Watch(callback func(FileEvent)) (io.Closer, error) {
	return watch(fs.Volume, callback)
}

func (v *volumeWrapper) Create(path string) (FileWriteCloser, error) {
	if !v.writable {
		return nil, fmt.Errorf("readonly %s", path)
	}
	if w, ok := v.Volume.(VolumeWriter); ok {
		return w.Create(path)
	}
	return nil, Unsupported
}

func (v *volumeWrapper) Remove(path string) error {
	if !v.writable {
		return fmt.Errorf("readonly %s", path)
	}
	if w, ok := v.Volume.(VolumeWriter); ok {
		return w.Remove(path)
	}
	return Unsupported
}
func (v *volumeWrapper) Mkdir(path string, perm os.FileMode) error {
	if !v.writable {
		return fmt.Errorf("readonly %s", path)
	}
	if w, ok := v.Volume.(VolumeWriter); ok {
		return w.Mkdir(path, perm)
	}
	return Unsupported
}

func (v *volumeWrapper) OpenFile(path string, flag int, perm os.FileMode) (File, error) {
	if !v.writable {
		return nil, fmt.Errorf("readonly %s", path)
	}
	if w, ok := v.Volume.(VolumeWriter); ok {
		return w.OpenFile(path, flag, perm)
	}
	return nil, Unsupported
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

func walkDir(v Volume, callback func(*FileInfo), path string) error {
	files, err := v.ReadDir(path)
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
			callback(f)
		}
	}
	return nil
}

func watch(v Volume, callback func(FileEvent)) (io.Closer, error) {
	if w, ok := v.(VolumeWatcher); ok {
		return w.Watch(callback)
	}
	return nil, Unsupported
}
