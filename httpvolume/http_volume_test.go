package httpvolume

import (
	"io/ioutil"
	"log"
	"os"
	"testing"
)

const testURL = "https://www.binzume.net/"

func init() {
	RequestLogger = log.New(os.Stderr, "", log.LstdFlags)
}

func TestHttpVolume(t *testing.T) {
	var vol = NewHTTPVolume(testURL, false)
	if !vol.Available() {
		t.Errorf("not available")
	}
}

func TestHttpVolume_Stat(t *testing.T) {
	var vol = NewHTTPVolume(testURL, false)

	stat, err := vol.Stat("index.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if stat.Size() == 0 {
		t.Errorf("empty content")
	}
	t.Logf("file: %v size: %v modified: %v", stat.Name(), stat.Size(), stat.ModTime())

	_, err = vol.Stat("notfound.txt")
	if err == nil {
		t.Errorf("unexpected error: %v", err)
	}

	var vol2 = NewHTTPVolume("", false)

	stat, err = vol2.Stat(testURL + "index.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if stat.Size() == 0 {
		t.Errorf("empty content")
	}
}

func TestHttpVolume_Open(t *testing.T) {
	var vol = NewHTTPVolume(testURL, false)

	r, err := vol.Open("index.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer r.Close()
	b, err := ioutil.ReadAll(r)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if string(b) == "" {
		t.Errorf("empty content: %v", string(b))
	}

	r, err = vol.Open("notfound.txt")
	if err == nil {
		t.Errorf("unexpected error: %v", err)
		r.Close()
	}
}

func TestHttpVolume_ReadAt(t *testing.T) {
	var vol = NewHTTPVolume(testURL, true)

	r, err := vol.Open("index.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer r.Close()
	b := make([]byte, 16)
	n, err := r.ReadAt(b, 24)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if n != len(b) {
		t.Errorf("length error: %v != %v", n, len(b))
	}

	if string(b) == "" {
		t.Errorf("empty content: %v", string(b))
	}
	t.Logf("content: %v", string(b))
}
