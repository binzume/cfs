package volume

import (
	"io/ioutil"
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
	var _ FS = vol

	testVolume(t, vol,
		[]string{"/hoge/test.txt", "/hoge/test.zip", "mem/hoge2/hello.txt"},
		[]string{"/not_existing_file", "/hoge/not_existing_dir/hello.txt"},
		[]string{"", "mem", "mem/hoge2", "/mem"},
		[]string{"/not_existing_dir", "/hoge/not_existing_dir"},
	)
	testVolumeWriter(t, vol,
		[]string{"hoge/created.txt"},
		[]string{"not_existing/test.txt"},
		[]string{},
		[]string{"not_existing/testdir", "mem/hoge2/testdir"},
	)

	vol.Clear()
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
