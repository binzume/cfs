package volume

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
)

type ZipVolume struct {
	volume Volume
	path   string
}

// NewZipVolume returns a new volume.
func NewZipVolume(path string, volume Volume) Volume {
	return &ZipVolume{volume: volume, path: path}
}

type zipFileReader struct {
	opener       func() (io.ReadCloser, error)
	parentCloser io.Closer
	size         int64
	reader       io.ReadCloser
	readPos      int64
	byteReader   *bytes.Reader
}

func (zfr *zipFileReader) Read(p []byte) (n int, err error) {
	if zfr.reader == nil {
		zfr.reader, err = zfr.opener()
		if err != nil {
			return
		}
	}
	n, err = zfr.reader.Read(p)
	zfr.readPos += int64(n)
	return
}

func (zfr *zipFileReader) Close() error {
	if zfr.reader != nil {
		zfr.reader.Close()
	}
	return zfr.parentCloser.Close()
}

func (zfr *zipFileReader) ReadAt(p []byte, off int64) (n int, err error) {
	if zfr.byteReader == nil {
		if off == zfr.readPos {
			return zfr.Read(p)
		}

		// load all
		// log.Printf("UNZIP READ ALL requested range: %v-%v size:(%v)", off, off+int64(len(p)), len(p))
		r, err := zfr.opener()
		if err != nil {
			return 0, err
		}
		defer r.Close()
		buf, err := ioutil.ReadAll(r)
		if err != nil {
			return 0, err
		}
		zfr.byteReader = bytes.NewReader(buf)
	}
	return zfr.byteReader.ReadAt(p, off)
}

func (v *ZipVolume) Available() bool {
	return true
}

func (v *ZipVolume) openZip() (io.Closer, *zip.Reader, error) {
	var fr FileReadCloser
	var err error
	var stat os.FileInfo

	if v.volume != nil {
		stat, err = v.volume.Stat(v.path)
	} else {
		stat, err = os.Stat(v.path)
	}
	if err != nil {
		return nil, nil, err
	}

	if v.volume != nil {
		fr, err = v.volume.Open(v.path)
	} else {
		fr, err = os.Open(v.path)
	}
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

func (v *ZipVolume) Stat(path string) (*FileInfo, error) {
	if path == "" {
		return &FileInfo{Path: path, FileMode: os.ModeDir}, nil
	}
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
			FileMode:    fi.Mode(),
			FileSize:    fi.Size(),
			UpdatedTime: fi.ModTime(),
		}, nil
	}
	return nil, noentError("Stat", path)
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
			FileMode:    fi.Mode(),
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
		opener := func() (io.ReadCloser, error) {
			return f.Open()
		}
		return &zipFileReader{opener: opener, parentCloser: closer, size: fi.Size()}, nil
	}
	closer.Close()
	return nil, noentError("Open", path)
}

const zipSep = ":"

type AutoUnzipVolume struct {
	FS
}

func NewAutoUnzipVolume(v Volume) FS {
	return &AutoUnzipVolume{ToFS(v)}
}

func (v *AutoUnzipVolume) Open(path string) (reader FileReadCloser, err error) {
	reader, err = v.FS.Open(path)
	if err == nil {
		return
	}
	pathAndName := strings.SplitN(path, "/"+zipSep+"/", 2)
	if len(pathAndName) == 2 && strings.HasSuffix(pathAndName[0], ".zip") {
		zv := &ZipVolume{v.FS, pathAndName[0]}
		return zv.Open(pathAndName[1])
	}
	return nil, err
}

func (v *AutoUnzipVolume) Stat(path string) (stat *FileInfo, err error) {
	stat, err = v.FS.Stat(path)
	if err == nil {
		return
	}
	pathAndName := strings.SplitN(path, "/"+zipSep+"/", 2)
	if len(pathAndName) == 2 && strings.HasSuffix(pathAndName[0], ".zip") {
		zv := &ZipVolume{v.FS, pathAndName[0]}
		stat, err := zv.Stat(pathAndName[1])
		if stat != nil {
			stat.Path = pathAndName[0] + "/" + zipSep + "/" + stat.Path
		}
		return stat, err
	}
	return nil, err
}

func (v *AutoUnzipVolume) ReadDir(path string) (files []*FileInfo, err error) {
	files, err = v.FS.ReadDir(path)
	if err == nil {
		return
	}
	pathAndName := strings.SplitN(path, "/"+zipSep+"/", 2)

	fi, err2 := v.Stat(pathAndName[0])
	if err2 == nil && !fi.IsDir() && strings.HasSuffix(pathAndName[0], ".zip") {
		zv := &ZipVolume{v.FS, pathAndName[0]}
		files, err = zv.ReadDir("")
		for _, fi := range files {
			fi.Path = zipSep + "/" + fi.Path
		}
	}
	return
}

func (v *AutoUnzipVolume) OpenFile(path string, flag int, mode os.FileMode) (File, error) {
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
	return v.FS.OpenFile(path, flag, mode)
}
