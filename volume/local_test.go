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

	testVolume(t, vol,
		[]string{"/test.txt", "/test.zip", "test.txt"},
		[]string{"/not_existing_file", "/not_existing_dir/hello.txt"},
		[]string{"/", "./"},
		[]string{"/not_existing_dir"},
	)
	testVolumeWriter(t, vol,
		[]string{"created.txt"},
		[]string{"not_existing/test.txt"},
		[]string{},
		[]string{"not_existing/testdir"},
	)
}

func TestLocalVolume_Stat(t *testing.T) {
	var vol = NewLocalVolume("./testdata")

	stat, err := vol.Stat("test.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if stat.Size() <= 0 {
		t.Errorf("unexpected size: %v", stat.Size())
	}

	if stat.ModTime().IsZero() {
		t.Errorf("ModTime is zero")
	}
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

func TestLocalVolume_Watch(t *testing.T) {
	var vol = NewLocalVolume("./testdata")

	c, err := vol.Watch(func(FileEvent) {})
	if err != nil {
		t.Errorf("error: %v", err)
	}
	defer c.Close()

}
