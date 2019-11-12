package volume

import (
	"archive/zip"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
)

type ZipVolume struct {
	Path string
	lock sync.Mutex
}

func NewZipVolume(path string) *ZipVolume {
	return &ZipVolume{Path: path}
}

type ZipFileReader struct {
	io.ReadCloser
	zipCloser io.Closer
}

func (zfr *ZipFileReader) Close() error {
	zfr.ReadCloser.Close()
	return zfr.zipCloser.Close()
}

func (zfr *ZipFileReader) ReadAt(p []byte, off int64) (n int, err error) {
	return 0, Unsupported
}

func (v *ZipVolume) Locker() sync.Locker {
	return &v.lock
}

func (v *ZipVolume) Available() bool {
	return true
}

func (v *ZipVolume) Stat(path string) (*FileStat, error) {
	return nil, Unsupported
}

func (v *ZipVolume) ReadDir(path string) ([]*FileEntry, error) {
	r, err := zip.OpenReader(v.Path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	files := []*FileEntry{}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		fe := &FileEntry{
			FileStat: FileStat{
				IsDir:       false,
				Size:        int64(f.UncompressedSize64),
				UpdatedTime: f.Modified,
			},
			Path: f.Name,
		}
		files = append(files, fe)
	}
	return files, nil
}

func (v *ZipVolume) Open(path string) (reader FileReadCloser, err error) {
	r, err := zip.OpenReader(v.Path)
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(f.Name) == path {
			fr, err := f.Open()
			if err != nil {
				r.Close()
				return nil, err
			}
			return &ZipFileReader{fr, r}, nil
		}
	}
	r.Close()
	return nil, errors.New("noent")
}

type ZipAsDirVolume struct {
	*LocalVolume
}

func (v *ZipAsDirVolume) Open(path string) (reader FileReadCloser, err error) {
	reader, err = v.LocalVolume.Open(path)
	if err == nil {
		return
	}
	pathAndName := strings.Split(path, "#")
	if len(pathAndName) == 2 && strings.HasSuffix(pathAndName[0], ".zip") {
		zv := &ZipVolume{Path: v.RealPath(pathAndName[0])}
		return zv.Open(pathAndName[1])
	}
	return nil, err
}

func (v *ZipAsDirVolume) ReadDir(path string) (files []*FileEntry, err error) {
	files, err = v.LocalVolume.ReadDir(path)
	if err == nil {
		return
	}

	fi, err2 := v.Stat(path)
	if err2 == nil && !fi.IsDir && strings.HasSuffix(path, ".zip") {
		localPath := v.RealPath(path)
		zv := &ZipVolume{Path: localPath}
		files, err = zv.ReadDir("")
		for _, fi := range files {
			fi.Path = path + "#/" + fi.Path
		}
	}
	return
}
