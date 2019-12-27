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
	ReadDir(path string) ([]*FileInfo, error)
	Stat(path string) (*FileInfo, error)
	Open(path string) (reader FileReadCloser, err error)
}

type VolumeWriter interface {
	Create(path string) (reader FileWriteCloser, err error)
	Mkdir(path string, mode os.FileMode) error
	OpenFile(path string, flag int, perm os.FileMode) (f File, err error)
	Remove(path string) error
}

type VolumeWalker interface {
	Walk(callback func(*FileInfo)) error
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

type FileInfo struct {
	Path        string                 `json:"path"`
	FileSize    int64                  `json:"size"`
	CreatedTime time.Time              `json:"created_time"`
	UpdatedTime time.Time              `json:"updated_time"`
	FileMode    os.FileMode            `json:"file_mode"`
	Metadata    map[string]interface{} `json:"metadata"`
}

func (f *FileInfo) Name() string {
	return filepath.Base(f.Path)
}

func (f *FileInfo) ModTime() time.Time {
	return f.UpdatedTime
}

func (f *FileInfo) Size() int64 {
	return f.FileSize
}

func (f *FileInfo) IsDir() bool {
	return f.FileMode&os.ModeDir != 0
}

func (f *FileInfo) Mode() os.FileMode {
	return f.FileMode
}

func (f *FileInfo) Sys() interface{} {
	return f.Metadata
}

type EventType int

const (
	CreateEvent EventType = iota
	RemoveEvent
	UpdateEvent
)

type FileEvent struct {
	Type EventType
	File *FileInfo
}

type FS interface {
	Volume
	VolumeWriter
	VolumeWalker
	VolumeWatcher
}

// utils
func SetMetadata(f *FileInfo, key string, value interface{}) *FileInfo {
	if f.Metadata == nil {
		f.Metadata = map[string]interface{}{}
	}
	f.Metadata[key] = value
	return f
}

func GetMetadata(f os.FileInfo, key string) interface{} {
	if fi, ok := f.(*FileInfo); ok && fi.Metadata != nil {
		return fi.Metadata[key]
	}
	return nil
}

var NoentError = errors.New("noent")
var PermissionError = errors.New("noent")
var UnsupportedError = errors.New("unsupported operation")

func noentError(op, path string) error {
	return &os.PathError{
		Op:   op,
		Path: path,
		Err:  NoentError,
	}
}

func permissionError(op, path string) error {
	return &os.PathError{
		Op:   op,
		Path: path,
		Err:  PermissionError,
	}
}

func unsupportedError(op, path string) error {
	return &os.PathError{
		Op:   op,
		Path: path,
		Err:  UnsupportedError,
	}
}
