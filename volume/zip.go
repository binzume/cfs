package volume

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
)

type ZipVolume struct {
	volume Volume
	path   string
}

func NewZipVolume(path string, volume Volume) *ZipVolume {
	return &ZipVolume{volume: volume, path: path}
}

type zipFileReader struct {
	io.ReadCloser
	zipCloser  io.Closer
	size       int64
	byteReader *bytes.Reader
}

func (zfr *zipFileReader) Close() error {
	zfr.ReadCloser.Close()
	return zfr.zipCloser.Close()
}

func (zfr *zipFileReader) ReadAt(p []byte, off int64) (n int, err error) {
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
	var size int64
	var fr FileReadCloser
	if v.volume != nil {
		stat, err := v.volume.Stat(v.path)
		if err != nil {
			return nil, nil, err
		}
		fr, err = v.volume.Open(v.path)
		if err != nil {
			return nil, nil, err
		}
		size = stat.Size()
	} else {
		stat, err := os.Stat(v.path)
		if err != nil {
			return nil, nil, err
		}
		fr, err = os.Open(v.path)
		if err != nil {
			return nil, nil, err
		}
		size = stat.Size()
	}
	r, err := zip.NewReader(fr, size)
	if err != nil {
		fr.Close()
		return nil, nil, err
	}
	return fr, r, err
}

func (v *ZipVolume) Stat(path string) (*FileInfo, error) {
	closer, r, err := v.openZip()
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	for _, f := range r.File {
		if !strings.HasSuffix("/"+f.Name, "/"+path) {
			continue
		}
		fi := f.FileInfo()
		return &FileInfo{
			Path:        f.Name,
			IsDirectory: fi.IsDir(),
			FileSize:    fi.Size(),
			UpdatedTime: fi.ModTime(),
		}, nil
	}
	return nil, errors.New("noent")
}

func (v *ZipVolume) ReadDir(path string) ([]*FileInfo, error) {
	closer, r, err := v.openZip()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	files := []*FileInfo{}
	for _, f := range r.File {
		fi := f.FileInfo()
		if fi.IsDir() {
			continue
		}
		fe := &FileInfo{
			Path:        f.Name,
			IsDirectory: fi.IsDir(),
			FileSize:    fi.Size(),
			UpdatedTime: fi.ModTime(),
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
		if fi.IsDir() || !strings.HasSuffix("/"+f.Name, "/"+path) {
			continue
		}
		fr, err := f.Open()
		if err != nil {
			closer.Close()
			return nil, err
		}
		return &zipFileReader{fr, closer, fi.Size(), nil}, nil
	}
	closer.Close()
	return nil, errors.New("noent")
}

type ZipAsDirVolume struct {
	FS
}

func (v *ZipAsDirVolume) Open(path string) (reader FileReadCloser, err error) {
	reader, err = v.FS.Open(path)
	if err == nil {
		return
	}
	pathAndName := strings.SplitN(path, "/:/", 2)
	if len(pathAndName) == 2 && strings.HasSuffix(pathAndName[0], ".zip") {
		zv := &ZipVolume{v.FS, pathAndName[0]}
		return zv.Open(pathAndName[1])
	}
	return nil, err
}

func (v *ZipAsDirVolume) Stat(path string) (stat *FileInfo, err error) {
	stat, err = v.FS.Stat(path)
	if err == nil {
		return
	}
	pathAndName := strings.SplitN(path, "/:/", 2)
	if len(pathAndName) == 2 && strings.HasSuffix(pathAndName[0], ".zip") {
		zv := &ZipVolume{v.FS, pathAndName[0]}
		return zv.Stat(pathAndName[1])
	}
	return nil, err
}

func (v *ZipAsDirVolume) ReadDir(path string) (files []*FileInfo, err error) {
	files, err = v.FS.ReadDir(path)
	if err == nil {
		return
	}

	fi, err2 := v.Stat(path)
	if err2 == nil && !fi.IsDir() && strings.HasSuffix(path, ".zip") {
		zv := &ZipVolume{v.FS, path}
		files, err = zv.ReadDir("")
		for _, fi := range files {
			fi.Path = ":/" + fi.Path
		}
	}
	return
}
