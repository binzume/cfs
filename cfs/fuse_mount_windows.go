package main

import (
	"log"
	"strings"
	"time"

	"github.com/keybase/dokan-go"
	"github.com/keybase/kbfs/dokan/winacl"

	"context"
)

// FileSystem
type fuseFs struct {
	v Volume
}

func (fs *fuseFs) WithContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctx, nil
}

func (fs *fuseFs) CreateFile(ctx context.Context, fi *dokan.FileInfo, cd *dokan.CreateData) (dokan.File, bool, error) {
	log.Println("CreateFile", fi.Path())
	path := strings.Replace(fi.Path()[1:], "\\", "/", -1)
	st, err := fs.v.Stat(path)
	if err != nil {
		return nil, false, err
	}
	return &fuseDir{v: fs.v, path: path, st: st}, st.IsDir, nil
}

func (fs *fuseFs) GetDiskFreeSpace(ctx context.Context) (dokan.FreeSpace, error) {
	log.Print("GetDiskFreeSpace")
	return dokan.FreeSpace{}, nil
}

func (fs *fuseFs) GetVolumeInformation(ctx context.Context) (dokan.VolumeInformation, error) {
	log.Print("GetVolumeInformation")
	return dokan.VolumeInformation{}, nil
}

func (fs *fuseFs) MoveFile(ctx context.Context, src dokan.File, sourceFI *dokan.FileInfo, targetPath string, replaceExisting bool) error {
	log.Println("MoveFile")
	return nil
}

func (fs *fuseFs) ErrorPrint(err error) {
	log.Print(err)
}

func debug(s string) {
	log.Println(s)
}

// File
type baseFile struct{}

func (f *baseFile) ReadFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	debug("ReadFile")
	return len(bs), nil
}
func (f *baseFile) WriteFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	return len(bs), nil
}
func (f *baseFile) FlushFileBuffers(ctx context.Context, fi *dokan.FileInfo) error {
	debug("FlushFileBuffers")
	return nil
}

func (f *baseFile) GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error) {
	debug("GetFileInformation")
	var st dokan.Stat
	st.FileAttributes = dokan.FileAttributeNormal
	return &st, nil
}
func (f *baseFile) FindFiles(context.Context, *dokan.FileInfo, string, func(*dokan.NamedStat) error) error {
	debug("FindFiles")
	return nil
}
func (f *baseFile) SetFileTime(context.Context, *dokan.FileInfo, time.Time, time.Time, time.Time) error {
	debug("SetFileTime")
	return nil
}
func (f *baseFile) SetFileAttributes(ctx context.Context, fi *dokan.FileInfo, fileAttributes dokan.FileAttribute) error {
	debug("SetFileAttributes")
	return nil
}

func (f *baseFile) LockFile(ctx context.Context, fi *dokan.FileInfo, offset int64, length int64) error {
	debug("LockFile")
	return nil
}
func (f *baseFile) UnlockFile(ctx context.Context, fi *dokan.FileInfo, offset int64, length int64) error {
	debug("UnlockFile")
	return nil
}

func (f *baseFile) SetEndOfFile(ctx context.Context, fi *dokan.FileInfo, length int64) error {
	debug("SetEndOfFile")
	return nil
}
func (f *baseFile) SetAllocationSize(ctx context.Context, fi *dokan.FileInfo, length int64) error {
	debug("SetAllocationSize")
	return nil
}

func (f *baseFile) CanDeleteFile(ctx context.Context, fi *dokan.FileInfo) error {
	// return dokan.ErrAccessDenied
	return nil
}
func (f *baseFile) CanDeleteDirectory(ctx context.Context, fi *dokan.FileInfo) error {
	return nil
}

func (f *baseFile) Cleanup(ctx context.Context, fi *dokan.FileInfo) {
	debug("Cleanup")
}

func (f *baseFile) CloseFile(ctx context.Context, fi *dokan.FileInfo) {
	debug("CloseFile")
}

func (f *baseFile) GetFileSecurity(ctx context.Context, fi *dokan.FileInfo, si winacl.SecurityInformation, sd *winacl.SecurityDescriptor) error {
	debug("GetFileSecurity")
	return nil
}
func (f *baseFile) SetFileSecurity(ctx context.Context, fi *dokan.FileInfo, si winacl.SecurityInformation, sd *winacl.SecurityDescriptor) error {
	debug("SetFileSecurity")
	return nil
}

var _ dokan.FileSystem = &fuseFs{}
var _ dokan.File = &baseFile{}

type fuseDir struct {
	baseFile
	path string
	v    Volume
	st   *FileStat
}

type FilesResponse struct {
	Files []*ChoiceFile `json:"files"`
	More  bool          `json:"more"`
}

type ChoiceFile struct {
	Id          string    `json:"id"`
	Url         string    `json:"url"`
	Name        string    `json:"name"`
	CreatedTime time.Time `json:"created_at"`
	UpdatedTime time.Time `json:"updated_at"`
}

func (t *fuseDir) FindFiles(ctx context.Context, fi *dokan.FileInfo, p string, cb func(*dokan.NamedStat) error) error {
	log.Println("FindFiles", fi.Path())
	files, err := t.v.ReadDir(t.path)
	for _, f := range files {
		st := dokan.NamedStat{}
		st.Name = f.Name
		st.FileSize = f.Size
		st.LastWrite = time.Unix(0, f.UpdatedTime)
		st.LastAccess = time.Unix(0, f.UpdatedTime)
		st.Creation = time.Unix(0, f.CreatedTime)
		if f.IsDir {
			st.FileAttributes = dokan.FileAttributeDirectory
		} else {
			st.FileAttributes = dokan.FileAttributeNormal
		}
		cb(&st)
	}
	return err
}

func (t *fuseDir) GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error) {
	debug("GetFileInformation " + t.path)
	if t.st == nil {
		f, err := t.v.Stat(t.path)
		if err != nil {
			return nil, err
		}
		t.st = f
	}
	f := t.st
	st := &dokan.Stat{
		FileSize:   f.Size,
		LastWrite:  time.Unix(0, f.UpdatedTime),
		LastAccess: time.Unix(0, f.UpdatedTime),
		Creation:   time.Unix(0, f.CreatedTime),
	}
	if f.IsDir {
		st.FileAttributes = dokan.FileAttributeDirectory
	}
	return st, nil
}

func (t *fuseDir) ReadFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	return t.v.Read(t.path, bs, offset)
}

func (t *fuseDir) WriteFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	t.st = nil
	return t.v.Write(t.path, bs, offset)
}

func (t *fuseDir) Cleanup(ctx context.Context, fi *dokan.FileInfo) {
	if fi.IsDeleteOnClose() {
		t.v.Remove(t.path)
	}
}

func fuseMount(v Volume, mountPoint string) {
	if len(mountPoint) > 2 {
		// q:hoge/fuga -> q: + hoge/fuga
		vg := NewVolumeGroup()
		vg.Add(mountPoint[2:], v)
		v = vg
		mountPoint = mountPoint[:2]
	}

	myFileSystem := &fuseFs{v: v}
	mp, err := dokan.Mount(&dokan.Config{FileSystem: myFileSystem, Path: mountPoint})
	if err != nil {
		log.Fatal("Mount failed:", err)
	}
	err = mp.BlockTillDone()
	if err != nil {
		log.Println("Filesystem exit:", err)
	}
}
