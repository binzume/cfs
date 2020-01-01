package httpvolume

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"time"

	"github.com/binzume/cfs/volume"
)

// RequestLogger is logger for debugging.
var RequestLogger *log.Logger

// UserAgent for HTTP request
var UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/74.0.3729.169 Safari/537.36 github.com/binzume/cfs/httpvolume"

type httpVolume struct {
	httpClient *http.Client
	baseURL    string
	lazyOpen   bool
}

// NewHTTPVolume returns a new volume. baseURL is optional.
func NewHTTPVolume(baseURL string, lazyOpen bool) volume.Volume {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Transport: &requestHandler{}}
	return &httpVolume{httpClient: client, baseURL: baseURL, lazyOpen: lazyOpen}
}

type requestHandler struct{}

func (t *requestHandler) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent)
	if RequestLogger != nil {
		RequestLogger.Println("REQUEST", req.Method, req.URL)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func (v *httpVolume) Available() bool {
	return true
}

func (v *httpVolume) Stat(path string) (*volume.FileInfo, error) {
	req, err := http.NewRequest("HEAD", v.getURL(path), nil)
	if err != nil {
		return nil, &os.PathError{Op: "Stat", Path: path, Err: err}
	}
	res, err := v.httpClient.Do(req)
	if err != nil {
		return nil, &os.PathError{Op: "Stat", Path: path, Err: err}
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return nil, &os.PathError{Op: "Stat", Path: path}
	}

	modifiedTime, _ := time.Parse(http.TimeFormat, res.Header.Get("Last-Modified"))
	return &volume.FileInfo{
		Path:        path,
		FileSize:    res.ContentLength,
		UpdatedTime: modifiedTime,
	}, nil
}

func (v *httpVolume) ReadDir(path string) ([]*volume.FileInfo, error) {
	return nil, volume.UnsupportedError
}

type httpReader struct {
	v       *httpVolume
	url     string
	body    io.ReadCloser
	bodyPos int64
}

func (hr *httpReader) open() error {
	req, err := http.NewRequest("GET", hr.url, nil)
	if err != nil {
		return &os.PathError{Op: "Open", Path: hr.url, Err: err}
	}
	res, err := hr.v.httpClient.Do(req)
	if err != nil {
		return &os.PathError{Op: "Open", Path: hr.url, Err: err}
	}
	if res.StatusCode >= 400 {
		res.Body.Close()
		return &os.PathError{Op: "Open", Path: hr.url}
	}
	hr.body = res.Body
	return nil
}

func (hr *httpReader) Read(p []byte) (n int, err error) {
	if hr.body == nil {
		err = hr.open()
		if err != nil {
			return
		}
	}
	n, err = hr.body.Read(p)
	hr.bodyPos += int64(n)
	return
}

func (hr *httpReader) Close() error {
	if hr.body != nil {
		return hr.body.Close()
	}
	return nil
}

func (hr *httpReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off == hr.bodyPos {
		return hr.Read(p)
	}
	req, err := http.NewRequest("GET", hr.url, nil)
	if err != nil {
		return 0, &os.PathError{Op: "ReadAt", Path: hr.url, Err: err}
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", off, off+int64(len(p))-1))
	res, err := hr.v.httpClient.Do(req)
	if err != nil {
		return 0, &os.PathError{Op: "ReadAt", Path: hr.url, Err: err}
	}
	defer res.Body.Close()
	n, err = res.Body.Read(p)
	if res.StatusCode >= 400 {
		err = &os.PathError{Op: "ReadAt", Path: hr.url}
	}
	return
}

func (v *httpVolume) Open(path string) (reader volume.FileReadCloser, err error) {
	url := v.getURL(path)
	hr := &httpReader{v: v, url: url}
	if v.lazyOpen {
		return hr, nil
	}
	err = hr.open()
	if err != nil {
		return nil, err
	}
	return hr, nil
}

func (v *httpVolume) getURL(vpath string) string {
	if v.baseURL == "" {
		return vpath
	}
	u, _ := url.Parse(v.baseURL)
	u.Path = path.Join(u.Path, vpath)
	return u.String()
}
