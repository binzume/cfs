package volume

import (
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"
)

func TestLocalVolume(t *testing.T) {
	var vol = NewLocalVolume("./testdata")
	var _ VolumeWriter = vol
	var _ VolumeWatcher = vol
	var _ VolumeWalker = vol

	testVolume(t, vol,
		[]string{"/test.txt", "/test.zip", "test.txt", "test/empty.txt"},
		[]string{"/not_existing_file", "/not_existing_dir/hello.txt"},
		[]string{"/", "", "test"},
		[]string{"/not_existing_dir"},
	)
	testVolumeWriter(t, vol,
		[]string{"created.txt"},
		[]string{"not_existing/test.txt"},
		[]string{},
		[]string{"not_existing/testdir"},
	)
}

func TestLocalVolume_Stat(t *testing.T) {
	var vol = NewLocalVolume("./testdata")

	stat, err := vol.Stat("test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if stat.Size() <= 0 {
		t.Errorf("unexpected size: %v", stat.Size())
	}

	if stat.ModTime().IsZero() {
		t.Errorf("ModTime is zero")
	}

	if data := stat.GetMetadata("meta"); data != nil {
		t.Errorf("Unexpected metadata: %v != nil", data)
	}
	if rstat := SetMetadata(stat, "meta", 123); rstat != stat {
		t.Errorf("SetMetadata should returns FileInfo")
	}
	if data := GetMetadata(stat, "meta").(int); data != 123 {
		t.Errorf("Unexpected metadata: %v != 123", data)
	}
	if _, ok := stat.Sys().(map[string]interface{}); !ok {
		t.Errorf("Metadata type error")
	}
	osstat, _ := os.Stat("./testdata")
	if rstat := SetMetadata(osstat, "meta", 123); rstat != nil {
		t.Errorf("SetMetadata error: returns != nil")
	}
	if GetMetadata(osstat, "meta") != nil {
		t.Errorf("Metadata error")
	}
}

func TestLocalVolume_Open(t *testing.T) {
	var vol = NewLocalVolume("./testdata")

	r, err := vol.Open("test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer r.Close()
	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if string(b) != "Hello" {
		t.Errorf("unexpexted string: %v", string(b))
	}
}

func TestLocalVolume_Watch(t *testing.T) {
	var vol = NewLocalVolume("./testdata")

	done := make(chan struct{})
	closeOnce := sync.Once{}
	c, err := vol.Watch(func(ev FileEvent) {
		t.Log(ev)
		closeOnce.Do(func() { close(done) })
	})
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer c.Close()
	vol.Mkdir("/test/dir", 777)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Log("watch timeout")
	}

	vol.Remove("/test/dir")
}
