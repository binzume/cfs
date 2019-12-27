package volume

import (
	"io/ioutil"
	"log"
	"testing"
)

func TestZipVolume(t *testing.T) {
	vol := NewZipVolume("testdata/test.zip", nil)

	testVolume(t, vol,
		[]string{"test.txt"},
		[]string{"not_existing_file", "not_existing_dir/test.txt"},
		[]string{""},
		[]string{},
	)
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
}

func TestAutoUnzipVolume_Open(t *testing.T) {
	var vol = NewAutoUnzipVolume(NewLocalVolume("./testdata"))

	testVolume(t, vol,
		[]string{"test.zip", "test.zip/:/test.txt"},
		[]string{"not_existing_file", "not_existing.zip/:/test.txt", "test.zip/:/not_existing"},
		[]string{"test.zip/:/", ""},
		[]string{"not_existing", "not_existing.zip/:/test.txt", "test.txt/:/hello"},
	)

	files, err := vol.ReadDir("test.zip")
	if err != nil {
		t.Errorf("error: %v", err)
	}

	for _, f := range files {
		log.Println(f)
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
