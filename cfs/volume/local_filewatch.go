package volume

import (
	"log"

	"github.com/fsnotify/fsnotify"
)

func (v *LocalVolume) StartScan() {
	v.Locker().Lock() // TODO
	defer v.Locker().Unlock()
	/*
		if v.scan {
			return
		}
		v.scan = true
		defer func() { v.scan = false }()
	*/
	go watch(v.LocalPath)
}

/*
func (v *Volume) AllFiles() []*File {
	v.lock.Lock() // TODO
	defer v.lock.Unlock()

	files := []*File{}
	for _, v := range v.Files {
		files = append(files, v)
	}
	return files
}
*/
func watch(path string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(path)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}
