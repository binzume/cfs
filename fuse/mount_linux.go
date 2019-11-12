package fuse

import (
	"log"

	"../volume"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

type fuseFs struct {
	pathfs.FileSystem
	v volume.Volume
}

type fuseFile struct {
	nodefs.File
	path  string
	v     volume.Volume
	fstat *volume.FileStat
}

func (t *fuseFs) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	f, err := t.v.Stat(name)
	if err != nil {
		return nil, fuse.ENOENT
	}

	if f.IsDir {
		return &fuse.Attr{
			Mode: fuse.S_IFDIR | 0755,
		}, fuse.OK
	}
	return &fuse.Attr{
		Mode:  fuse.S_IFREG | 0644,
		Size:  uint64(f.Size),
		Ctime: uint64(f.CreatedTime),
		Mtime: uint64(f.UpdatedTime),
		Atime: uint64(f.UpdatedTime),
	}, fuse.OK
}

func (t *fuseFs) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	files, err := t.v.ReadDir(name)
	if err != nil {
		return nil, fuse.ENOENT
	}

	result := []fuse.DirEntry{}
	for _, f := range files {
		result = append(result, fuse.DirEntry{Name: f.Name, Mode: fuse.S_IFREG})
	}

	return result, fuse.OK
}

func (t *fuseFs) Open(name string, flags uint32, context *fuse.Context) (file nodefs.File, code fuse.Status) {
	f, err := t.v.Stat(name)
	if err != nil {
		return nil, fuse.ENOENT
	}
	if flags&fuse.O_ANYWRITE != 0 {
		return nil, fuse.EPERM
	}

	return &fuseFile{File: nodefs.NewDefaultFile(), fstat: f, v: t.v, path: name}, fuse.OK
}

func (f *fuseFile) Read(buf []byte, off int64) (fuse.ReadResult, fuse.Status) {
	len, err := f.v.Read(f.path, buf, off)
	if err != nil {
		return nil, fuse.ENOSYS
	}

	return fuse.ReadResultData(buf[:len]), fuse.OK
}

func (f *fuseFile) Write(data []byte, off int64) (uint32, fuse.Status) {
	len, err := f.v.Write(f.path, data, off)
	if err != nil {
		return 0, fuse.ENOSYS
	}
	return uint32(len), fuse.OK
}

func MountVolume(v volume.FS, mountPoint string) <-chan error {

	nfs := pathfs.NewPathNodeFs(&fuseFs{FileSystem: pathfs.NewDefaultFileSystem(), v: v}, nil)
	server, _, err := nodefs.MountRoot(mountPoint, nfs.Root(), nil)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	go server.Serve()
	return make(chan error)
}
