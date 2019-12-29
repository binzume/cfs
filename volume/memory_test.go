package volume

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestOnMemoryVolume(t *testing.T) {
	vol := NewOnMemoryVolume(map[string][]byte{
		"hello.txt": []byte("Hello"),
		"hoge.txt":  []byte("World"),
	})
	testVolume(t, vol,
		[]string{"hello.txt", "hoge.txt"},
		[]string{"not_existing_file", "not_existing_dir/hello.txt"},
		[]string{""},
		[]string{},
	)
}

func TestOnMemoryVolume_Open(t *testing.T) {
	var vol Volume = NewOnMemoryVolume(map[string][]byte{
		"hello.txt": []byte("Hello"),
	})

	r, err := vol.Open("hello.txt")
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

func TestOnMemoryVolume_Stat(t *testing.T) {
	var vol Volume = NewOnMemoryVolume(map[string][]byte{
		"hello.txt": []byte("Hello"),
		"hoge.txt":  []byte("World"),
	})

	stat, err := vol.Stat("hello.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}
	if stat.Size() != 5 {
		t.Errorf("size: %v", stat.Size())
	}
}

func TestOnMemoryVolume_Remove(t *testing.T) {
	var vol = NewOnMemoryVolume(map[string][]byte{
		"hello.txt": []byte("Hello"),
		"hoge.txt":  []byte("World"),
	})

	err := vol.Remove("hello.txt")
	if err != nil {
		t.Errorf("error: %v", err)
	}

	_, err = vol.Stat("hello.txt")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}
}
