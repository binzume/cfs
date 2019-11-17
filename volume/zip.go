package volume

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ZipVolume struct {
	Path string
}

func NewZipVolume(path string) *ZipVolume {
	return &ZipVolume{Path: path}
}

type ZipFileReader struct {
	io.ReadCloser
	zipCloser  io.Closer
	size       int64
	byteReader *bytes.Reader
}

func (zfr *ZipFileReader) Close() error {
	zfr.ReadCloser.Close()
	return zfr.zipCloser.Close()
}

func (zfr *ZipFileReader) ReadAt(p []byte, off int64) (n int, err error) {
	// TODO: check position
	if zfr.byteReader == nil {
		buf := new(bytes.Buffer)
		sz, err := io.Copy(buf, zfr)
		if err != nil {
			return 0, err
		}
		if sz != zfr.size {
			// maybe already Read() has been called...
			return 0, Unsupported
		}
		zfr.byteReader = bytes.NewReader(buf.Bytes())
	}
	return zfr.byteReader.ReadAt(p, off)
}

func (v *ZipVolume) Available() bool {
	return true
}

func (v *ZipVolume) openZip() (io.Closer, *zip.Reader, error) {
	stat, err := os.Stat(v.Path)
	if err != nil {
		return nil, nil, err
	}
	fr, err := os.Open(v.Path)
	if err != nil {
		return nil, nil, err
	}
	r, err := zip.NewReader(fr, stat.Size())
	if err != nil {
		fr.Close()
		return nil, nil, err
	}
	return fr, r, err
}

func (v *ZipVolume) Stat(path string) (*FileStat, error) {
	closer, r, err := v.openZip()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	for _, f := range r.File {
		if filepath.Base(f.Name) != path {
			continue
		}
		fi := f.FileInfo()
		return &FileStat{
			IsDir:       fi.IsDir(),
			Size:        fi.Size(),
			UpdatedTime: fi.ModTime(),
		}, nil
	}
	return nil, errors.New("noent")
}

func (v *ZipVolume) ReadDir(path string) ([]*FileEntry, error) {
	closer, r, err := v.openZip()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	files := []*FileEntry{}
	for _, f := range r.File {
		fi := f.FileInfo()
		if fi.IsDir() {
			continue
		}
		fe := &FileEntry{
			FileStat: FileStat{
				IsDir:       fi.IsDir(),
				Size:        fi.Size(),
				UpdatedTime: fi.ModTime(),
			},
			Path: f.Name,
		}
		files = append(files, fe)
	}
	return files, nil
}

func (v *ZipVolume) Open(path string) (reader FileReadCloser, err error) {
	closer, r, err := v.openZip()
	if err != nil {
		return nil, err
	}

	for _, f := range r.File {
		fi := f.FileInfo()
		if fi.IsDir() {
			continue
		}
		if filepath.Base(f.Name) == path {
			fr, err := f.Open()
			if err != nil {
				closer.Close()
				return nil, err
			}
			return &ZipFileReader{fr, closer, fi.Size(), nil}, nil
		}
	}
	closer.Close()
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
	pathAndName := strings.SplitN(path, "/:/", 2)
	if len(pathAndName) == 2 && strings.HasSuffix(pathAndName[0], ".zip") {
		zv := &ZipVolume{Path: v.RealPath(pathAndName[0])}
		return zv.Open(pathAndName[1])
	}
	return nil, err
}

func (v *ZipAsDirVolume) Stat(path string) (stat *FileStat, err error) {
	stat, err = v.LocalVolume.Stat(path)
	if err == nil {
		return
	}
	pathAndName := strings.SplitN(path, "/:/", 2)
	if len(pathAndName) == 2 && strings.HasSuffix(pathAndName[0], ".zip") {
		zv := &ZipVolume{Path: v.RealPath(pathAndName[0])}
		return zv.Stat(pathAndName[1])
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
			fi.Path = ":/" + fi.Path
		}
	}
	return
}
