package fuse2

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestMount(t *testing.T) {
	fsys := os.DirFS("../volume/testdata")

	go func() {
		err := <-MountVolume(fsys, "X:")

		if err != nil {
			t.Errorf("error: %v", err)
		}
	}()

	time.Sleep(1 * time.Second)

	data, err := ioutil.ReadFile("X:/test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}

	if string(data) != "Hello" {
		t.Errorf("unexpected: %v", string(data))
	}

	time.Sleep(5 * time.Second)
}
