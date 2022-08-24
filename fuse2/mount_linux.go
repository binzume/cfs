package fuse2

import (
	"io"
	"io/fs"
	"log"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

type fuseFs struct {
	pathfs.FileSystem
	fsys fs.FS
}

type fuseFile struct {
	nodefs.File
	path  string
	fsys  fs.FS
	fstat fs.FileInfo
}

func (t *fuseFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	f, err := fs.Stat(t.fsys, name)
	if err != nil {
		return nil, fuse.ENOENT
	}

	if f.IsDir() {
		return &fuse.Attr{
			Mode: fuse.S_IFDIR | 0755,
		}, fuse.OK
	}
	return &fuse.Attr{
		Mode:  fuse.S_IFREG | 0644,
		Size:  uint64(f.Size()),
		Ctime: uint64(f.ModTime().Unix()),
		Mtime: uint64(f.ModTime().Unix()),
		Atime: uint64(f.ModTime().Unix()),
	}, fuse.OK
}

func (t *fuseFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	files, err := fs.ReadDir(t.fsys, name)
	if err != nil {
		return nil, fuse.ENOENT
	}

	result := []fuse.DirEntry{}
	for _, f := range files {
		result = append(result, fuse.DirEntry{Name: f.Name(), Mode: fuse.S_IFREG})
	}

	return result, fuse.OK
}

func (t *fuseFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	f, err := fs.Stat(t.fsys, name)
	if err != nil {
		return nil, fuse.ENOENT
	}
	if flags&fuse.O_ANYWRITE != 0 {
		return nil, fuse.EPERM
	}

	return &fuseFile{File: nodefs.NewDefaultFile(), fstat: f, fsys: t.fsys, path: name}, fuse.OK
}

func (f *fuseFile) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	ff, err := f.fsys.Open(f.path)
	if err != nil {
		return nil, fuse.ENOSYS
	}
	defer ff.Close()

	len, err := ReadAt(ff, buf, off)
	if err != nil {
		return nil, fuse.ENOSYS
	}

	return fuse.ReadResultData(buf[:len]), fuse.OK
}

func (f *fuseFile) Write(data []byte, off int64) (uint32, fuse.Status) {
	fsys, ok := f.fsys.(interface {
		OpenWriter(string) (io.WriteCloser, error)
	})
	if !ok {
		return 0, fuse.ENOSYS
	}

	ff, err := fsys.OpenWriter(f.path)
	if err != nil {
		return 0, fuse.ENOSYS
	}
	defer ff.Close()

	len, err := WriteAt(ff, data, off)
	if err != nil {
		return 0, fuse.ENOSYS
	}
	return uint32(len), fuse.OK
}

type handle struct {
	nfs    *pathfs.PathNodeFs
	server *fuse.Server
}

func (h *handle) Close() error {
	return h.server.Unmount()
}

func MountFS(mountPoint string, fsys fs.FS, opt interface{}) (io.Closer, error) {
	nfs := pathfs.NewPathNodeFs(&fuseFs{FileSystem: pathfs.NewDefaultFileSystem(), fsys: fsys}, nil)
	server, _, err := nodefs.MountRoot(mountPoint, nfs.Root(), nil)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	go server.Serve()
	server.WaitMount()
	return &handle{nfs: nfs, server: server}, err
}
