package volume

import (
	"io/ioutil"
	"log"
	"os"
	"syscall"
	"testing"
)

func TestVolumeWrapper(t *testing.T) {
	var vol = NewOnMemoryVolume(map[string][]byte{
		"hello.txt": []byte("Hello"),
		"hoge.txt":  []byte("World"),
	})

	var fs FS = ToFS(vol)
	_, err := fs.Create("hoge.txt")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	err = fs.Mkdir("hoge.txt", 666)
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	err = fs.Remove("hoge.txt")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	_, err = fs.OpenFile("hoge.txt", syscall.O_WRONLY, 0)
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	_, err = fs.Watch(func(f FileEvent) {})
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	// open readonly
	r, err := fs.OpenFile("hoge.txt", syscall.O_RDONLY, 0)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer r.Close()
	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if string(b) != "World" {
		t.Errorf("unexpexted string: %v", string(b))
	}

	err = fs.Walk(func(f *FileInfo) {
		log.Println(f)
	})
	if err != nil {
		t.Errorf("error: %v", err)
	}
}
