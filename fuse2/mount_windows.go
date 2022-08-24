package fuse2

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"strings"
	"syscall"
	"time"

	"github.com/keybase/kbfs/dokan"
	"github.com/keybase/kbfs/dokan/winacl"
)

func fileATime(fi fs.FileInfo) time.Time {
	if attr, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, attr.LastAccessTime.Nanoseconds())
	}
	return fi.ModTime()
}

func fileCTime(fi fs.FileInfo) time.Time {
	if attr, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, attr.CreationTime.Nanoseconds())
	}
	return fi.ModTime()
}

// FileSystem
type fuseFs struct {
	fsys fs.FS
}

func (ffs *fuseFs) WithContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return ctx, nil
}

func (ffs *fuseFs) CreateFile(ctx context.Context, fi *dokan.FileInfo, cd *dokan.CreateData) (dokan.File, dokan.CreateStatus, error) {
	path := strings.TrimLeft(strings.Replace(fi.Path()[1:], "\\", "/", -1), "/")
	if path == "" {
		path = "."
	}
	if cd.CreateDisposition == dokan.FileCreate {
		fsys, ok := ffs.fsys.(interface {
			Create(name string) (fs.File, error)
		})
		if !ok {
			return nil, 0, fs.ErrInvalid
		}
		file, err := fsys.Create(path)
		if err != nil {
			return nil, dokan.FileNonDirectoryFile, err
		}
		return &fuseFile{v: ffs.fsys, path: path, st: nil, file: file}, dokan.NewFile, nil
	}
	st, err := fs.Stat(ffs.fsys, path)
	if err != nil {
		return nil, 0, err
	}
	if st.IsDir() {
		return &fuseFile{v: ffs.fsys, path: path, st: st}, dokan.ExistingDir, nil
	}
	var file io.Closer
	if cd.DesiredAccess == 0x17019F { // TODO
		if fsys, ok := ffs.fsys.(interface {
			OpenWriter(string) (io.WriteCloser, error)
		}); ok {
			file, err = fsys.OpenWriter(path)
		}
	} else {
		file, err = ffs.fsys.Open(path)
	}
	if err != nil {
		return nil, 0, err
	}
	return &fuseFile{v: ffs.fsys, path: path, st: st, file: file}, dokan.ExistingFile, nil
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

func (*fuseFs) Printf(format string, v ...interface{}) {}

var _ dokan.FileSystem = &fuseFs{}
var _ dokan.File = &baseFile{}

type fuseFile struct {
	baseFile
	path string
	v    fs.FS
	st   fs.FileInfo
	file io.Closer
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

func (t *fuseFile) FindFiles(ctx context.Context, fi *dokan.FileInfo, p string, cb func(*dokan.NamedStat) error) error {
	files, err := fs.ReadDir(t.v, t.path)
	for _, f := range files {
		st := dokan.NamedStat{}
		info, _ := f.Info()
		st.Name = f.Name()
		st.FileSize = info.Size()
		st.LastWrite = info.ModTime()
		st.LastAccess = fileATime(info)
		st.Creation = fileCTime(info)
		if f.IsDir() {
			st.FileAttributes = dokan.FileAttributeDirectory
		} else {
			st.FileAttributes = dokan.FileAttributeNormal
		}
		cb(&st)
	}
	return err
}

func (t *fuseFile) GetFileInformation(ctx context.Context, fi *dokan.FileInfo) (*dokan.Stat, error) {
	if t.st == nil {
		f, err := fs.Stat(t.v, t.path)
		if err != nil {
			return nil, err
		}
		t.st = f
	}
	f := t.st
	st := &dokan.Stat{
		FileSize:   f.Size(),
		LastWrite:  f.ModTime(),
		LastAccess: fileATime(f),
		Creation:   fileCTime(f),
	}
	if f.IsDir() {
		st.FileAttributes = dokan.FileAttributeDirectory
	}
	return st, nil
}

func (t *fuseFile) ReadFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	return ReadAt(t.file, bs, offset)
}

func (t *fuseFile) WriteFile(ctx context.Context, fi *dokan.FileInfo, bs []byte, offset int64) (int, error) {
	t.st = nil // ignore cache
	return WriteAt(t.file, bs, offset)
}

func (f *fuseFile) CloseFile(ctx context.Context, fi *dokan.FileInfo) {
	if f.file != nil {
		f.file.Close()
		f.file = nil
	}
}

func (t *fuseFile) Cleanup(ctx context.Context, fi *dokan.FileInfo) {
	if fi.IsDeleteOnClose() {
		if v, ok := t.v.(interface{ Remove(name string) error }); ok {
			v.Remove(t.path)
		}
	}
}

func MountVolume(fsys fs.FS, mountPoint string) <-chan error {
	errorch := make(chan error)
	go func() {
		myFileSystem := &fuseFs{fsys: fsys}
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
