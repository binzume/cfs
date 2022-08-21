package volume

import (
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func TestZipVolume_Open(t *testing.T) {
	var vol = NewFS("../volume/testdata/test.zip", nil)

	r, err := vol.Open("test.txt")
	if err != nil {
		t.Fatalf("error: %v", err)
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
	var vol = NewFS("../volume/testdata/test.zip", nil)

	stat, err := vol.Stat("test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if stat.Size() == 0 {
		t.Errorf("unexpexted size: %v", stat.Size())
	}
}

func TestAutoUnzipVolume_Open(t *testing.T) {
	var vol = NewAutoUnzipFS(os.DirFS("../volume/testdata"))

	files, err := fs.ReadDir(vol, "test.zip")
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
	var vol = NewFS("../volume/testdata/test.zip", nil)

	{
		// sequential read
		r, err := vol.Open("test.txt")
		if err != nil {
			t.Errorf("error: %v", err)
		}
		defer r.Close()
		ra := r.(io.ReaderAt)
		b := make([]byte, 2)

		if n, err := ra.ReadAt(b, 0); err != nil || n != len(b) {
			t.Errorf("ReadAt error: %v, read: %v", err, n)
		}
		if n, err := ra.ReadAt(b, 2); err != nil || n != len(b) {
			t.Errorf("ReadAt error: %v, read: %v", err, n)
		}
	}

	{
		// random read
		r, err := vol.Open("test.txt")
		if err != nil {
			t.Errorf("error: %v", err)
		}
		defer r.Close()
		ra := r.(io.ReaderAt)
		b := make([]byte, 2)
		if n, err := ra.ReadAt(b, 2); err != nil || n != len(b) {
			t.Errorf("ReadAt error: %v, read: %v", err, n)
		}
		if n, err := ra.ReadAt(b, 0); err != nil || n != len(b) {
			t.Errorf("ReadAt error: %v, read: %v", err, n)
		}
	}
}
