package wsvolume

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/binzume/cfs/volume"
	"github.com/gorilla/websocket"
)

func TestWsVolume(t *testing.T) {
	vol := NewWebsocketVolume("hoge")

	// implment volume reader/writer
	var _ volume.Volume = vol
	var _ volume.VolumeWriter = vol
}

func TestWsVolume_Connect1(t *testing.T) {
	vol := NewWebsocketVolume("hoge")
	provider := NewWebsocketVolumeProvider(volume.NewLocalVolume("../volume/testdata"))

	connected := make(chan struct{})
	once := sync.Once{}
	wsupgrader := websocket.Upgrader{}
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsupgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go provider.HandleSession(conn, "file")
		once.Do(func() { close(connected) })
	})
	testServer := httptest.NewServer(testHandler)
	defer testServer.Close()

	wsurl := "ws" + strings.TrimPrefix(testServer.URL, "http")

	// Connect
	t.Logf("connecting.. %v", wsurl)
	err := vol.StartClientWithDefaultConnector(wsurl)
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer vol.Terminate()
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout")
	}

	files, err := vol.ReadDir("")
	if err != nil {
		t.Errorf("ReadDir error: %v", err)
	}
	for _, f := range files {
		t.Log(f)
	}

	r, err := vol.Open("test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer r.Close()
	b := make([]byte, 5)
	n, err := r.Read(b)
	if err != nil || n != len(b) {
		t.Errorf("error: %v", err)
	}
	if string(b) != "Hello" {
		t.Errorf("unexpexted string: %v", string(b))
	}
}

func TestWsVolume_Connect2(t *testing.T) {
	vol := NewWebsocketVolume("hoge")
	provider := NewWebsocketVolumeProvider(volume.NewLocalVolume("../volume/testdata"))

	connected := make(chan struct{})
	once := sync.Once{}
	wsupgrader := websocket.Upgrader{}
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsupgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		vol.BindConnection(conn)
		once.Do(func() { close(connected) })
	})
	testServer := httptest.NewServer(testHandler)
	defer testServer.Close()

	wsurl := "ws" + strings.TrimPrefix(testServer.URL, "http")

	// Connect
	t.Logf("connecting.. %v", wsurl)
	provider.StartClientWithDefaultConnector(wsurl)
	defer provider.Terminate()
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout")
	}

	files, err := vol.ReadDir("")
	if err != nil {
		t.Errorf("ReadDir error: %v", err)
	}
	for _, f := range files {
		t.Log(f)
	}

	stat, err := vol.Stat("test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if stat.Name() != "test.txt" {
		t.Errorf("name error: %v", stat.Name())
	}
	if stat.Size() == 0 {
		t.Errorf("size error: %v", stat.Size())
	}

	r, err := vol.Open("test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer r.Close()
	b := make([]byte, 5)
	n, err := r.Read(b)
	if err != nil || n != len(b) {
		t.Errorf("error: %v", err)
	}
	if string(b) != "Hello" {
		t.Errorf("unexpexted string: %v", string(b))
	}

	n, err = r.ReadAt(b, 0)
	if err != nil || n != len(b) {
		t.Errorf("error: %v", err)
	}
	if string(b) != "Hello" {
		t.Errorf("unexpexted string: %v", string(b))
	}
}
