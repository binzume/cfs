package fuse

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/keybase/dokan-go"
	"github.com/keybase/kbfs/dokan/winacl"

	"github.com/binzume/cfs/volume"
)

// FileSystem
type fuseFs struct {
	v volume.FS
}

func (fs *fuseFs) WithContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctx, nil
}

func (fs *fuseFs) CreateFile(ctx context.Context, fi *dokan.FileInfo, cd *dokan.CreateData) (dokan.File, bool, error) {
	path := strings.TrimLeft(strings.Replace(fi.Path()[1:], "\\", "/", -1), "/")
	if cd.CreateDisposition == dokan.FileCreate {
		file, err := fs.v.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0)
		if err != nil {
			return nil, false, err
		}
		_, err = file.WriteAt([]byte{}, 0)
		if err != nil {
			return nil, false, err
		}
		return &fuseDir{v: fs.v, path: path, st: nil, file: file}, false, nil
	}
	st, err := fs.v.Stat(path)
	if err != nil {
		return nil, false, err
	}
	var file volume.File
	if !st.IsDir() {
		flag := 0
		if cd.DesiredAccess == 0x17019F { // TODO
			flag = os.O_RDWR
		}
		file, err = fs.v.OpenFile(path, flag, 0)
		if err != nil {
			return nil, false, err
		}
	}
	return &fuseDir{v: fs.v, path: path, st: st, file: file}, st.IsDir(), nil
}

func (fs *fuseFs) GetDiskFreeSpace(ctx context.Context) (dokan.FreeSpace, error) {
	// log.Print("GetDiskFreeSpace")
	var sz uint64 = 1024 * 1024 * 1024 * 100 // 100GB
	return dokan.FreeSpace{FreeBytesAvailable: sz, TotalNumberOfBytes: sz, TotalNumberOfFreeBytes: sz}, nil
}

func (fs *fuseFs) GetVolumeInformation(ctx context.Context) (dokan.VolumeInformation, error) {
	// log.Print("GetVolumeInformation")
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
	return 0, fmt.Errorf("unsupported operation")
}
func (f *baseFile) WriteFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	return 0, fmt.Errorf("unsupported operation")
}
func (f *baseFile) FlushFileBuffers(ctx context.Context, fi *dokan.FileInfo) error {
	return nil
}

func (f *baseFile) GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error) {
	var st dokan.Stat
	st.FileAttributes = dokan.FileAttributeNormal
	return &st, nil
}
func (f *baseFile) FindFiles(context.Context, *dokan.FileInfo, string, func(*dokan.NamedStat) error) error {
	return fmt.Errorf("unsupported operation")
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
	return nil
}
func (f *baseFile) UnlockFile(ctx context.Context, fi *dokan.FileInfo, offset int64, length int64) error {
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
	v    volume.FS
	st   *volume.FileInfo
	file volume.File
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
	files, err := t.v.ReadDir(t.path)
	for _, f := range files {
		st := dokan.NamedStat{}
		st.Name = f.Name()
		st.FileSize = f.Size()
		st.LastWrite = f.ModTime()
		st.LastAccess = f.ModTime()
		st.Creation = f.CreatedTime
		if f.IsDir() {
			st.FileAttributes = dokan.FileAttributeDirectory
		} else {
			st.FileAttributes = dokan.FileAttributeNormal
		}
		cb(&st)
	}
	return err
}

func (t *fuseDir) GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error) {
	if t.st == nil {
		f, err := t.v.Stat(t.path)
		if err != nil {
			return nil, err
		}
		t.st = f
	}
	f := t.st
	st := &dokan.Stat{
		FileSize:   f.FileSize,
		LastWrite:  f.UpdatedTime,
		LastAccess: f.UpdatedTime,
		Creation:   f.CreatedTime,
	}
	if f.IsDir() {
		st.FileAttributes = dokan.FileAttributeDirectory
	}
	return st, nil
}

func (t *fuseDir) ReadFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	if t.file == nil {
		file, err := t.v.OpenFile(t.path, 0, 0)
		if err != nil {
			return 0, err
		}
		t.file = file
	}
	return t.file.ReadAt(bs, offset)
}

func (t *fuseDir) WriteFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	t.st = nil
	return t.file.WriteAt(bs, offset)
}

func (t *fuseDir) Cleanup(ctx context.Context, fi *dokan.FileInfo) {
	if fi.IsDeleteOnClose() {
		t.v.Remove(t.path)
	}
}

func MountVolume(v volume.Volume, mountPoint string) <-chan error {
	_, err := os.Stat(mountPoint)
	if len(mountPoint) > 2 && os.IsNotExist(err) {
		// q:hoge/fuga -> q: + hoge/fuga
		vg := volume.NewVolumeGroup()
		vg.AddVolume(filepath.ToSlash(mountPoint[2:]), v)
		v = vg
		mountPoint = mountPoint[:2]
	}

	errorch := make(chan error)
	go func() {
		myFileSystem := &fuseFs{v: volume.ToFS(v)}
		mp, err := dokan.Mount(&dokan.Config{FileSystem: myFileSystem, Path: mountPoint})
		if err != nil {
			errorch <- err
			log.Fatal("Mount failed:", err)
		}
		err = mp.BlockTillDone()
		errorch <- err
	}()
	return errorch
}
