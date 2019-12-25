package volume

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Volume interface {
	Available() bool
	VolumeReader
}

type VolumeReader interface {
	ReadDir(path string) ([]*FileEntry, error)
	Stat(path string) (*FileStat, error)
	Open(path string) (reader FileReadCloser, err error)
}

type VolumeWriter interface {
	Create(path string) (reader FileWriteCloser, err error)
	Mkdir(path string, mode os.FileMode) error
	OpenFile(path string, flag int, perm os.FileMode) (f File, err error)
	Remove(path string) error
}

type VolumeWalker interface {
	Walk(callback func(*FileEntry)) error
}

type VolumeWatcher interface {
	Watch(callback func(FileEvent)) (io.Closer, error)
}

type FileReadCloser interface {
	io.ReadCloser
	io.ReaderAt
}

type FileWriteCloser interface {
	io.WriteCloser
	io.WriterAt
}

type File interface {
	io.ReadWriteCloser
	io.ReaderAt
	io.WriterAt
}

type FileStat struct {
	Size        int64     `json:"size"`
	CreatedTime time.Time `json:"created_time"`
	UpdatedTime time.Time `json:"updated_time"`
	IsDir       bool      `json:"is_directory"`
}

type FileEntry struct {
	Path string `json:"path"`
	FileStat
}

func (f *FileEntry) Name() string {
	return filepath.Base(f.Path)
}

func (f *FileStat) ModTime() time.Time {
	return f.UpdatedTime
}

func (f *FileStat) Sys() interface{} {
	return nil
}

type EventType int

var Unsupported = errors.New("unsupported operation")

const (
	CreateEvent EventType = iota
	RemoveEvent
	UpdateEvent
)

type FileEvent struct {
	Type EventType
	File *FileEntry
}

type FS interface {
	Volume
	VolumeWriter
	VolumeWalker
	VolumeWatcher
}
