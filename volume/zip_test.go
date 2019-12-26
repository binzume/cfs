package volume

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestZipVolume(t *testing.T) {
	var vol Volume = NewZipVolume("testdata/test.zip", nil)

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
	var vol = NewZipVolume("testdata/test.zip", nil)

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

func TestZipVolume_Stat(t *testing.T) {
	var vol = NewZipVolume("testdata/test.zip", nil)

	stat, err := vol.Stat("test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if stat.Size() == 0 {
		t.Errorf("unexpexted size: %v", stat.Size())
	}

	_, err = vol.Stat("notfound.txt")
	if err == nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAutoUnzipVolume_Open(t *testing.T) {
	var vol = NewAutoUnzipVolume(NewLocalVolume("./testdata"))

	files, err := vol.ReadDir("test.zip")
	if err != nil {
		t.Errorf("error: %v", err)
	}

	for _, f := range files {
		log.Println(f)
	}

	_, err = vol.Stat("test.zip/:/test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}

	r, err := vol.Open("test.zip/:/test.txt")
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

func TestHttpVolume_ReadAt(t *testing.T) {
	var vol = NewZipVolume("testdata/test.zip", nil)

	r, err := vol.Open("test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer r.Close()
	b := make([]byte, 4)
	n, err := r.ReadAt(b, 1)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if n != len(b) {
		t.Errorf("length error: %v != %v", n, len(b))
	}
}
