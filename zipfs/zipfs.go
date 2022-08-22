package zipfs

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"strings"
)

type ZipFS struct {
	fsys fs.FS
	path string
}

// NewFS returns a new FS. (fsys = nil : native path)
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
		return nil, nil, &fs.PathError{Op: "open", Path: v.path, Err: errors.New("ReaderAt not implemented")}
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
		stat, err := fs.Stat(v.fsys, v.path)
		if stat != nil {
			stat = &modeDirOverride{stat}
		}
		return stat, err
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
		return &fileEntry{FileInfo: f.FileInfo(), rawName: f.Name}, nil
	}
	return nil, fs.ErrNotExist
}

type fileEntry struct {
	rawName string
	fs.FileInfo
}

func (f *fileEntry) Name() string {
	return f.rawName
}

func (f *fileEntry) Type() fs.FileMode {
	return f.Mode().Type()
}

func (f *fileEntry) Info() (fs.FileInfo, error) {
	return f, nil
}

type modeDirOverride struct {
	fs.FileInfo
}

func (f *modeDirOverride) IsDir() bool {
	return true
}

type modeDirOverrideDirEnt struct {
	fs.DirEntry
}

func (f *modeDirOverrideDirEnt) IsDir() bool {
	return true
}

func (f *modeDirOverrideDirEnt) Info() (fs.FileInfo, error) {
	info, err := f.DirEntry.Info()
	if info != nil {
		info = &modeDirOverride{info}
	}
	return info, err
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
		fe := &fileEntry{
			rawName:  f.Name,
			FileInfo: fi,
		}
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
		return &zipFileReader{opener: opener, parentCloser: closer, size: fi.Size(), stat: fi}, nil
	}
	closer.Close()
	return nil, fs.ErrNotExist
}

type AutoUnzipFS struct {
	fs.FS
	ModeDir   bool
	zipPrefix string
}

func NewAutoUnzipFS(fsys fs.FS) fs.FS {
	return &AutoUnzipFS{FS: fsys, ModeDir: true, zipPrefix: ":/"}
}

func (v *AutoUnzipFS) IsZipFile(path string) bool {
	return strings.HasSuffix(path, ".zip") || strings.HasSuffix(path, ".ZIP")
}

func (v *AutoUnzipFS) Open(path string) (reader fs.File, err error) {
	reader, err = v.FS.Open(path)
	if err == nil {
		return
	}
	pathAndName := strings.SplitN(path, "/"+v.zipPrefix, 2)
	if len(pathAndName) == 2 && v.IsZipFile(pathAndName[0]) {
		zv := &ZipFS{v.FS, pathAndName[0]}
		return zv.Open(pathAndName[1])
	}
	return nil, err
}

func (v *AutoUnzipFS) Stat(path string) (stat fs.FileInfo, err error) {
	stat, err = fs.Stat(v.FS, path)
	if err == nil {
		if v.ModeDir && !stat.IsDir() && v.IsZipFile(stat.Name()) {
			stat = &modeDirOverride{stat}
		}
		return
	}
	pathAndName := strings.SplitN(path, "/"+v.zipPrefix, 2)
	if len(pathAndName) == 2 && v.IsZipFile(pathAndName[0]) {
		zv := &ZipFS{v.FS, pathAndName[0]}
		stat, err := zv.Stat(pathAndName[1])
		if stat, ok := stat.(*fileEntry); ok {
			stat.rawName = v.zipPrefix + stat.rawName
		}
		return stat, err
	}
	return nil, err
}

func (v *AutoUnzipFS) ReadDir(path string) (files []fs.DirEntry, err error) {
	files, err = fs.ReadDir(v.FS, path)
	if err == nil {
		if v.ModeDir {
			for i := range files {
				if !files[i].IsDir() && v.IsZipFile(files[i].Name()) {
					files[i] = &modeDirOverrideDirEnt{files[i]}
				}
			}
		}
		return
	}
	pathAndName := strings.SplitN(path, "/"+v.zipPrefix, 2)
	if !v.IsZipFile(pathAndName[0]) {
		return
	}
	fi, err2 := fs.Stat(v.FS, pathAndName[0])
	if err2 == nil && !fi.IsDir() {
		zv := &ZipFS{v.FS, pathAndName[0]}
		files, err = zv.ReadDir("")
		for _, fi := range files {
			if fi, ok := fi.(*fileEntry); ok {
				fi.rawName = v.zipPrefix + fi.rawName
			}
		}
	}
	return
}
