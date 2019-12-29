package volume

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type LocalVolume struct {
	basePath string
}

// NewLocalVolume returns a new volume.
func NewLocalVolume(basePath string) *LocalVolume {
	return &LocalVolume{basePath: basePath}
}

func newLocalFileEntry(path string, info os.FileInfo) *FileInfo {
	return &FileInfo{
		Path:        path,
		FileSize:    info.Size(),
		UpdatedTime: info.ModTime(),
		CreatedTime: time.Unix(0, GetCTime(info)),
		FileMode:    info.Mode(),
	}
}

func (v *LocalVolume) Available() bool {
	_, err := os.Stat(v.basePath)
	return err == nil
}

func (v *LocalVolume) Stat(path string) (*FileInfo, error) {
	fi, err := os.Stat(v.RealPath(path))
	if err != nil {
		return nil, err
	}
	return newLocalFileEntry(path, fi), nil
}

func (v *LocalVolume) ReadDir(path string) ([]*FileInfo, error) {
	items, err := ioutil.ReadDir(v.RealPath(path))
	if err != nil {
		return nil, err
	}
	files := []*FileInfo{}
	for _, fi := range items {
		files = append(files, newLocalFileEntry(fi.Name(), fi))
	}
	return files, err
}

func (v *LocalVolume) RealPath(path string) string {
	// Real path should be included in basePath.
	return filepath.Join(v.basePath, filepath.Join("/", path))
}

func (v *LocalVolume) OpenFile(path string, flag int, perm os.FileMode) (f File, err error) {
	return os.OpenFile(v.RealPath(path), flag, perm)
}

func (v *LocalVolume) Open(path string) (reader FileReadCloser, err error) {
	return os.Open(v.RealPath(path))
}

func (v *LocalVolume) Create(path string) (reader FileWriteCloser, err error) {
	return os.Create(v.RealPath(path))
}

func (v *LocalVolume) Remove(path string) error {
	return os.Remove(v.RealPath(path))
}

func (v *LocalVolume) Mkdir(path string, mode os.FileMode) error {
	return os.Mkdir(v.RealPath(path), mode)
}

func (v *LocalVolume) Walk(callback func(*FileInfo)) error {
	return v.walk(callback, "")
}

func (v *LocalVolume) walk(callback func(*FileInfo), path string) error {
	f := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		vpath, _ := filepath.Rel(v.basePath, path)
		callback(newLocalFileEntry(vpath, info))
		return nil
	}
	return filepath.Walk(v.RealPath(path), f)
}

func (v *LocalVolume) Watch(callback func(FileEvent)) (io.Closer, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	go func() {
		done := make(chan bool)
		go func() {
			defer watcher.Close()
			for {
				select {
				case event := <-watcher.Events:
					log.Println("event:", event)
					path, _ := filepath.Rel(v.basePath, event.Name)
					if (event.Op & fsnotify.Write) != 0 {
						info, err := os.Stat(event.Name)
						if err == nil {
							callback(FileEvent{UpdateEvent, newLocalFileEntry(path, info)})
						}
					}
					if (event.Op & fsnotify.Create) != 0 {
						info, err := os.Stat(event.Name)
						if err == nil {
							if info.IsDir() {
								watcher.Add(event.Name)
								v.walk(func(f *FileInfo) {
									callback(FileEvent{CreateEvent, f})
								}, path)
							} else {
								callback(FileEvent{CreateEvent, newLocalFileEntry(path, info)})
							}
						}
					}
					if (event.Op & (fsnotify.Remove | fsnotify.Rename)) != 0 {
						callback(FileEvent{RemoveEvent, &FileInfo{Path: path}})
					}
				case err := <-watcher.Errors:
					log.Println("error:", err)
					done <- false
				}
			}
		}()

		f := func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				err = watcher.Add(path)
				if err != nil {
					log.Println("error:", err, path)
					// done <- false
				}
				return nil
			}
			// fch <- NewLocalFile(path, info, v)
			return nil
		}

		filepath.Walk(v.basePath, f)
		// close(fch)
		if err != nil {
			log.Fatal(err)
		}
		<-done
	}()
	return watcher, nil
}
