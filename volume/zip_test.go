package volume

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestZipVolume(t *testing.T) {
	var vol Volume = NewZipVolume("testdata/test.zip")

	if !vol.Available() {
		t.Fatal("volume is unavailable")
	}

	files, err := vol.ReadDir("")
	if err != nil {
		t.Errorf("error: %v", err)
	}

	for _, f := range files {
		log.Println(f)
	}
}

func TestZipVolume_Open(t *testing.T) {
	var vol = NewZipVolume("testdata/test.zip")

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

	_, err = vol.Open("notfound.txt")
	if err == nil {
		t.Errorf("unexpected error: %v", err)
	}
}
