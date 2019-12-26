package http_volume

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"path"

	"github.com/binzume/cfs/volume"
)

var RequestLogger *log.Logger = nil
var UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/74.0.3729.169 Safari/537.36"

func NewHttpClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	return &http.Client{Jar: jar, Transport: &requestHandler{}}, err
}

type requestHandler struct{}

func (t *requestHandler) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent)
	if RequestLogger != nil {
		RequestLogger.Println("REQUEST", req.Method, req.URL)
	}
	return http.DefaultTransport.RoundTrip(req)
}

type HttpVolume struct {
	HttpClient *http.Client
	baseURL    string
	lazyOpen   bool
}

func NewHttpVolume(baseUrl string, lazy bool) volume.Volume {
	client, err := NewHttpClient()
	if err != nil {
		panic(err)
	}

	return &HttpVolume{HttpClient: client, baseURL: baseUrl, lazyOpen: lazy}
}

func (v *HttpVolume) Available() bool {
	return true
}

func (v *HttpVolume) Stat(path string) (*volume.FileInfo, error) {
	req, err := http.NewRequest("HEAD", v.getUrl(path), nil)
	if err != nil {
		return nil, err
	}
	res, err := v.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// TODO: Last-Modified header
	return &volume.FileInfo{
		Path:     path,
		FileSize: res.ContentLength,
	}, nil
}

func (v *HttpVolume) ReadDir(path string) ([]*volume.FileInfo, error) {
	return nil, volume.UnsupportedError
}

type httpReader struct {
	v    *HttpVolume
	url  string
	body io.ReadCloser
}

func (hr *httpReader) open() error {
	req, err := http.NewRequest("GET", hr.url, nil)
	if err != nil {
		return err
	}
	res, err := hr.v.HttpClient.Do(req)
	if err != nil {
		return err
	}
	hr.body = res.Body
	return nil
}

func (hr *httpReader) Read(p []byte) (n int, err error) {
	if hr.body == nil {
		err = hr.open()
		if err != nil {
			return 0, err
		}
	}
	return hr.body.Read(p)
}

func (hr *httpReader) Close() error {
	if hr.body != nil {
		return hr.body.Close()
	}
	return nil
}

func (hr *httpReader) ReadAt(p []byte, off int64) (n int, err error) {
	req, err := http.NewRequest("GET", hr.url, nil)
	if err != nil {
		return
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%v-%v", off, off+int64(len(p))-1))
	res, err := hr.v.HttpClient.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	n, err = res.Body.Read(p)
	if n == len(p) && err == io.EOF {
		err = nil // Body.Read returns EOF error when completed.
	}
	return
}

func (v *HttpVolume) Open(path string) (reader volume.FileReadCloser, err error) {
	url := v.getUrl(path)
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

func (v *HttpVolume) getUrl(vpath string) string {
	if v.baseURL == "" {
		return vpath
	}
	u, _ := url.Parse(v.baseURL)
	u.Path = path.Join(u.Path, vpath)
	return u.String()
}
