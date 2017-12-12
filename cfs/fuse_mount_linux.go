package main

import (
	"log"

	"./volume"

	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
)

// TODO implemente read/write.

type fuseFs struct {
	pathfs.FileSystem
	v volume.Volume
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
		Mode: fuse.S_IFREG | 0644, Size: uint64(f.Size),
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

	b := make([]byte, f.Size)
	return nodefs.NewDataFile(b), fuse.OK
}

func fuseMount(v volume.Volume, mountPoint string) <-chan error {

	nfs := pathfs.NewPathNodeFs(&fuseFs{FileSystem: pathfs.NewDefaultFileSystem(), v: v}, nil)
	server, _, err := nodefs.MountRoot(mountPoint, nfs.Root(), nil)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}
	server.Serve()
	return make(chan error)
}
