package volume

import (
	"io/ioutil"
	"log"
	"testing"
)

func newTestVolumeGroup() *VolumeGroup {
	vol1 := NewLocalVolume("./testdata")
	vol2 := NewOnMemoryVolume(map[string][]byte{
		"hello.txt": []byte("Hello"),
		"hoge.txt":  []byte("World"),
	})
	vol := NewVolumeGroup()
	vol.AddVolume("hoge", vol1)
	vol.AddVolume("mem/hoge2", vol2)
	return vol
}

func TestVolumeGroup(t *testing.T) {
	var vol = newTestVolumeGroup()
	var _ VolumeWriter = vol
	var _ VolumeWatcher = vol
	var _ VolumeWalker = vol
}

func TestVolumeGroup_Stat(t *testing.T) {
	var vol Volume = newTestVolumeGroup()

	st, err := vol.Stat("")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !st.IsDir {
		t.Fatalf("stat err: %v", st)
	}

	st, err = vol.Stat("hoge/test.txt")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if st.Size == 0 {
		t.Fatalf("stat err: %v", st)
	}

	st, err = vol.Stat("mem/hoge2")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !st.IsDir {
		t.Fatalf("stat err: %v", st)
	}
}

func TestVolumeGroup_Open(t *testing.T) {
	vol := newTestVolumeGroup()

	r, err := vol.Open("mem/hoge2/hello.txt")
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

func TestVolumeGroup_ReadDir(t *testing.T) {
	vol := newTestVolumeGroup()

	files, err := vol.ReadDir("mem/hoge2")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	for _, f := range files {
		log.Println(f)
	}
}

func TestVolumeGroup_Walk(t *testing.T) {
	vgroup := newTestVolumeGroup()
	vgroup.Walk(func(f *FileEntry) {
		log.Println(f)
	})
}
