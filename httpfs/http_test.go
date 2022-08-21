package httpfs

import (
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func init() {
	RequestLogger = log.New(os.Stderr, "", log.LstdFlags)
}

func TestHttpVolume_Stat(t *testing.T) {
	testHandler := http.NewServeMux()
	testHandler.Handle("/index.txt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "0123456789abcdef")
	}))
	testServer := httptest.NewServer(testHandler)
	defer testServer.Close()

	var vol = NewFS("", false)

	stat, err := vol.Stat(testServer.URL + "/index.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if stat.Size() != 16 {
		t.Errorf("invalid size %v", stat.Size())
	}
	t.Logf("file: %v size: %v modified: %v", stat.Name(), stat.Size(), stat.ModTime())

	_, err = vol.Stat(testServer.URL + "/notfound.txt")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	_, err = vol.Stat("invalid\nurl")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}
	_, err = vol.Stat("nothttp://hello")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	var vol2 = NewFS(testServer.URL, false)
	stat, err = vol2.Stat("index.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if stat.Size() != 16 {
		t.Errorf("invalid size %v", stat.Size())
	}

}

func TestHttpVolume_Open(t *testing.T) {
	testHandler := http.NewServeMux()
	testHandler.Handle("/index.txt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "0123456789abcdef")
	}))
	testServer := httptest.NewServer(testHandler)
	defer testServer.Close()

	var vol = NewFS("", false)

	r, err := vol.Open(testServer.URL + "/index.txt")
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

	_, err = vol.Open(testServer.URL + "/notfound.txt")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}
	_, err = vol.Open("invalid\nurl")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}
	_, err = vol.Open("nothttp://hello")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	// lazy
	var vol3 = NewFS("", true)
	r, err = vol3.Open("notfound.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer r.Close()
	_, err = ioutil.ReadAll(r)
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}
}

func TestHttpVolume_ReadDir(t *testing.T) {
	testHandler := http.NewServeMux()
	testHandler.Handle("/index.txt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "0123456789abcdef")
	}))
	testServer := httptest.NewServer(testHandler)
	defer testServer.Close()

	var vol = NewFS(testServer.URL, false)

	// TODO
	_, _ = fs.ReadDir(vol, "")
}

func TestHttpVolume_ReadAt(t *testing.T) {
	reqCount := 0
	testHandler := http.NewServeMux()
	testHandler.Handle("/index.txt", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount++
		io.WriteString(w, "0123456789abcdef")
	}))
	testServer := httptest.NewServer(testHandler)
	defer testServer.Close()

	var vol = NewFS("", true)

	r, err := vol.Open(testServer.URL + "/index.txt")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	defer r.Close()
	if reqCount != 0 {
		t.Errorf("reqCount: %v", reqCount)
	}
	ra := r.(io.ReaderAt)

	// sequential read
	b := make([]byte, 6)
	offsets := []int64{0, 6, 12}
	errs := []error{nil, nil, io.EOF}
	contents := []string{"012345", "6789ab", "cdef"}
	for i, ofs := range offsets {
		n, err := ra.ReadAt(b, ofs)
		if err != errs[i] {
			t.Errorf("error: %v", err)
		}
		if string(b[:n]) != contents[i] {
			t.Errorf("unexpected content: %v", string(b))
		}
		if reqCount != 1 {
			t.Errorf("should not increase reqCount: %v", reqCount)
		}
	}

	// random access
	_, err = ra.ReadAt(b, 3)
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if reqCount != 2 {
		t.Errorf("reqCount: %v", reqCount)
	}

	// error
	r, err = vol.Open("invalid\nurl")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer r.Close()
	ra = r.(io.ReaderAt)
	_, err = ra.ReadAt(b, 1)
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

}
