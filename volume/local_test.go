package volume

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestLocalVolume(t *testing.T) {
	var vol = NewLocalVolume("./testdata")
	var _ VolumeWriter = vol
	var _ VolumeWatcher = vol
	var _ VolumeWalker = vol
}
func TestLocalVolume_ReadDir(t *testing.T) {
	var vol = NewLocalVolume("./testdata")

	files, err := vol.ReadDir("")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	for _, f := range files {
		log.Println(f)
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
