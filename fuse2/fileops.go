package fuse2

import (
	"io"
	"io/fs"
)

func ReadAt(f io.Closer, b []byte, off int64) (int, error) {
	if f, ok := f.(io.ReaderAt); ok {
		return f.ReadAt(b, off)
	}
	if f, ok := f.(io.ReadSeeker); ok {
		_, err := f.Seek(off, io.SeekStart)
		if err != nil {
			return 0, err
		}
		return io.ReadFull(f, b)
	}
	return 0, fs.ErrInvalid
}

func WriteAt(f io.Closer, b []byte, off int64) (int, error) {
	if f, ok := f.(io.WriterAt); ok {
		return f.WriteAt(b, off)
	}
	if f, ok := f.(io.WriteSeeker); ok {
		_, err := f.Seek(off, io.SeekStart)
		if err != nil {
			return 0, err
		}
		return f.Write(b)
	}
	return 0, fs.ErrInvalid
}

func Truncate(fsys fs.FS, name string, size int64) error {
	s, ok := fsys.(interface {
		Truncate(string, int64) error
	})
	if !ok {
		return fs.ErrInvalid
	}
	return s.Truncate(name, size)
}
