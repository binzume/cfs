package fuse

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/binzume/cfs/volume"
)

func TestMount(t *testing.T) {
	var vol volume.Volume = volume.NewOnMemoryVolume(map[string][]byte{
		"hello.txt": []byte("Hello"),
		"hoge.txt":  []byte("World"),
	})

	go func() {
		err := <-MountVolume(vol, "X:")

		if err != nil {
			t.Errorf("error: %v", err)
		}
	}()

	time.Sleep(1 * time.Second)

	data, err := ioutil.ReadFile("X:/hello.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}

	if string(data) != "Hello" {
		t.Errorf("unexpected: %v", string(data))
	}

	time.Sleep(5 * time.Second)
}
