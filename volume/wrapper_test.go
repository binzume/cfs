package volume

import (
	"os"
	"testing"
)

func TestVolumeWrapper(t *testing.T) {
	var vol = NewOnMemoryVolume(map[string][]byte{
		"hello.txt": []byte("Hello"),
		"hoge.txt":  []byte("World"),
	})
	var fs FS = ToFS(vol)

	testVolume(t, fs,
		[]string{"hello.txt"},
		[]string{"aaaa"},
		[]string{""},
		[]string{},
	)
	testVolumeWriter(t, fs,
		[]string{},
		[]string{"create.txt"},
		[]string{},
		[]string{"testdir"},
	)

	err := fs.Remove("hoge.txt")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	maskedFs := &struct {
		Volume
		VolumeWriter
	}{fs, fs}
	testVolumeWriter(t, ToFS(maskedFs),
		[]string{},
		[]string{"create.txt"},
		[]string{},
		[]string{"testdir"},
	)
}
