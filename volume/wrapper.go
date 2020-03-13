package volume

import (
	"io"
	"os"
	"path"
	"syscall"
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

func UnwrapVolume(v Volume) Volume {
	if wv, ok := v.(*volumeWrapper); ok {
		return wv.Volume
	}
	return v
}

func (fs *volumeWrapper) Walk(callback func(*FileInfo)) error {
	return walk(fs.Volume, callback)
}

func (fs *volumeWrapper) Watch(callback func(FileEvent)) (io.Closer, error) {
	return watch(fs.Volume, callback)
}

func (v *volumeWrapper) Create(path string) (FileWriteCloser, error) {
	if w, ok := v.Volume.(VolumeWriter); ok && v.writable {
		return w.Create(path)
	}
	return nil, permissionError("Create", path)
}

func (v *volumeWrapper) Remove(path string) error {
	if w, ok := v.Volume.(VolumeWriter); ok && v.writable {
		return w.Remove(path)
	}
	return permissionError("Remove", path)
}
func (v *volumeWrapper) Mkdir(path string, perm os.FileMode) error {
	if w, ok := v.Volume.(VolumeWriter); ok && v.writable {
		return w.Mkdir(path, perm)
	}
	return permissionError("Mkdir", path)
}

func (v *volumeWrapper) OpenFile(path string, flag int, perm os.FileMode) (File, error) {
	if flag == syscall.O_RDONLY {
		f, err := v.Open(path)
		if err != nil {
			return nil, err
		}
		return &struct {
			FileReadCloser
			io.WriterAt
			io.Writer
		}{f, nil, nil}, nil
	}
	if w, ok := v.Volume.(VolumeWriter); ok && v.writable {
		return w.OpenFile(path, flag, perm)
	}
	return nil, permissionError("OpenFile", path)
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
		f.Path = path.Join(p, f.Path)
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
	return nil, unsupportedError("Watch", "")
}
