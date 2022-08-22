package zipfs

import (
	"archive/zip"
	"bytes"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"strings"

	"github.com/binzume/cfs/volume"
)

type ZipFS struct {
	fsys fs.FS
	path string
}

// NewFS returns a new FS.
func NewFS(path string, fsys fs.FS) fs.StatFS {
	return &ZipFS{fsys: fsys, path: path}
}

type zipFileReader struct {
	opener       func() (io.ReadCloser, error)
	parentCloser io.Closer
	size         int64
	reader       io.ReadCloser
	readPos      int64
	byteReader   *bytes.Reader
	stat         fs.FileInfo
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

func (zfr *zipFileReader) Stat() (fs.FileInfo, error) {
	return zfr.stat, nil
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

func (v *ZipFS) Available() bool {
	return true
}

func (v *ZipFS) openZip() (io.Closer, *zip.Reader, error) {
	var fr fs.File
	var err error
	var stat fs.FileInfo

	if v.fsys != nil {
		stat, err = fs.Stat(v.fsys, v.path)
	} else {
		stat, err = os.Stat(v.path)
	}
	if err != nil {
		return nil, nil, err
	}

	if v.fsys != nil {
		fr, err = v.fsys.Open(v.path)
	} else {
		fr, err = os.Open(v.path)
	}
	if err != nil {
		return nil, nil, err
	}

	readerAt, ok := fr.(io.ReaderAt)
	if !ok {
		return nil, nil, volume.UnsupportedError
	}

	r, err := zip.NewReader(readerAt, stat.Size())
	if err != nil {
		fr.Close()
		return nil, nil, err
	}
	return fr, r, err
}

func (v *ZipFS) Stat(path string) (fs.FileInfo, error) {
	if path == "" {
		return &volume.FileInfo{Path: path, FileMode: os.ModeDir}, nil
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
		return &volume.FileInfo{
			Path:        f.Name,
			FileMode:    fi.Mode(),
			FileSize:    fi.Size(),
			UpdatedTime: fi.ModTime(),
		}, nil
	}
	return nil, fs.ErrNotExist
}

type fileEntry struct {
	volume.FileInfo
}

func (f *fileEntry) Type() os.FileMode {
	return f.FileMode
}

func (f *fileEntry) Info() (fs.FileInfo, error) {
	return &f.FileInfo, nil
}

func (v *ZipFS) ReadDir(path string) ([]fs.DirEntry, error) {
	closer, r, err := v.openZip()
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	files := []fs.DirEntry{}
	for _, f := range r.File {
		fi := f.FileInfo()
		if fi.IsDir() {
			continue
		}
		fe := &fileEntry{volume.FileInfo{
			Path:        f.Name,
			FileMode:    fi.Mode(),
			FileSize:    fi.Size(),
			UpdatedTime: fi.ModTime(),
		}}
		files = append(files, fe)
	}
	return files, nil
}

func (v *ZipFS) Open(path string) (reader fs.File, err error) {
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
		stat := &volume.FileInfo{
			Path:        f.Name,
			FileMode:    fi.Mode(),
			FileSize:    fi.Size(),
			UpdatedTime: fi.ModTime(),
		}
		return &zipFileReader{opener: opener, parentCloser: closer, size: fi.Size(), stat: stat}, nil
	}
	closer.Close()
	return nil, fs.ErrNotExist
}

const zipSep = ":"

type AutoUnzipFS struct {
	fs.FS
}

func NewAutoUnzipFS(fsys fs.FS) fs.FS {
	return &AutoUnzipFS{FS: fsys}
}

func (v *AutoUnzipFS) IsZipFile(path string) bool {
	return strings.HasSuffix(path, ".zip") || strings.HasSuffix(path, ".ZIP")
}

func (v *AutoUnzipFS) Open(path string) (reader fs.File, err error) {
	reader, err = v.FS.Open(path)
	if err == nil {
		return
	}
	pathAndName := strings.SplitN(path, "/"+zipSep+"/", 2)
	if len(pathAndName) == 2 && v.IsZipFile(pathAndName[0]) {
		zv := &ZipFS{v.FS, pathAndName[0]}
		return zv.Open(pathAndName[1])
	}
	return nil, err
}

func (v *AutoUnzipFS) Stat(path string) (stat fs.FileInfo, err error) {
	stat, err = fs.Stat(v.FS, path)
	if err == nil {
		return
	}
	pathAndName := strings.SplitN(path, "/"+zipSep+"/", 2)
	if len(pathAndName) == 2 && v.IsZipFile(pathAndName[0]) {
		zv := &ZipFS{v.FS, pathAndName[0]}
		stat, err := zv.Stat(pathAndName[1])
		if stat, ok := stat.(*volume.FileInfo); ok {
			stat.Path = pathAndName[0] + "/" + zipSep + "/" + stat.Path
		}
		return stat, err
	}
	return nil, err
}

func (v *AutoUnzipFS) ReadDir(path string) (files []fs.DirEntry, err error) {
	files, err = fs.ReadDir(v.FS, path)
	if err == nil {
		return
	}
	pathAndName := strings.SplitN(path, "/"+zipSep+"/", 2)

	fi, err2 := v.Stat(pathAndName[0])
	if err2 == nil && !fi.IsDir() && v.IsZipFile(pathAndName[0]) {
		zv := &ZipFS{v.FS, pathAndName[0]}
		files, err = zv.ReadDir("")
		for _, fi := range files {
			if fi, ok := fi.(*fileEntry); ok {
				fi.Path = zipSep + "/" + fi.Path
			}
		}
	}
	return
}
