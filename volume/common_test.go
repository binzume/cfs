package volume

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func testVolume(t *testing.T, vol Volume, okFiles, errorFiles, okDirs, errorDirs []string) {

	if !vol.Available() {
		t.Fatal("volume is unavailable")
	}

	for _, fpath := range okFiles {
		stat, err := vol.Stat(fpath)
		if err != nil {
			t.Errorf("Stat error. path: %v, error: %v", fpath, err)
		}
		if stat.Name() != filepath.Base(fpath) {
			t.Errorf("unexpected name: %v path: %v", stat.Name(), fpath)
		}
		if stat.IsDir() {
			t.Errorf("Stat should not dir: %v", fpath)
		}

		r, err := vol.Open(fpath)
		if err != nil {
			t.Errorf("Open error. path: %v, error: %v", fpath, err)
		} else {
			r.Close()
		}

		if w, ok := vol.(VolumeWriter); ok {
			// read only
			r, err := w.OpenFile(fpath, syscall.O_RDONLY, 0)
			if err != nil {
				t.Errorf("OpenFile error. path: %v, error: %v", fpath, err)
			} else {
				r.Close()
			}
		}
	}

	for _, fpath := range errorFiles {
		_, err := vol.Stat(fpath)
		if _, ok := err.(*os.PathError); !ok {
			t.Errorf("Stat should return pathError. path: %v err: %v", fpath, err)
		}

		_, err = vol.Open(fpath)
		if _, ok := err.(*os.PathError); !ok {
			t.Errorf("Open should return pathError. path: %v err: %v", fpath, err)
		}

		if w, ok := vol.(VolumeWriter); ok {
			_, err := w.OpenFile(fpath, syscall.O_RDONLY, 0)
			if _, ok := err.(*os.PathError); !ok {
				t.Errorf("OpenFile should return pathError. path: %v err: %v", fpath, err)
			}
		}
	}

	for _, fpath := range okDirs {
		stat, err := vol.Stat(fpath)
		if err != nil {
			t.Errorf("Stat error. path: %v, error: %v", fpath, err)
		}
		if stat.Name() != filepath.Base(fpath) {
			t.Errorf("unexpected name: %v path: %v", stat.Name(), fpath)
		}
		if !stat.IsDir() {
			t.Errorf("Stat should dir: %v", fpath)
		}
		if stat.Mode()&os.ModeDir == 0 {
			t.Errorf("Stat mode should ModeDir: %v", fpath)
		}

		files, err := vol.ReadDir(fpath)
		if err != nil {
			t.Errorf("ReadDir error. path: %v, error: %v", fpath, err)
		} else {
			t.Logf("%v contains %v files.", fpath, len(files))
		}
	}

	for _, fpath := range errorDirs {
		_, err := vol.Stat(fpath)
		if _, ok := err.(*os.PathError); !ok {
			t.Errorf("Stat should return pathError. path: %v err: %v", fpath, err)
		}

		_, err = vol.ReadDir(fpath)
		if _, ok := err.(*os.PathError); !ok {
			t.Errorf("ReadDir should return pathError. path: %v err: %v", fpath, err)
		}
	}

	if w, ok := vol.(VolumeWalker); ok {
		err := w.Walk(func(*FileInfo) {})
		if err != nil {
			t.Errorf("Walk error: %v", err)
		}
	}

	if w, ok := vol.(VolumeWatcher); ok {
		c, err := w.Watch(func(FileEvent) {})
		if perr, ok := err.(*os.PathError); ok {
			// maybe unsupported
			if perr.Err != UnsupportedError {
				t.Errorf("Watch error: %v", err)
			}
		} else if err != nil {
			t.Errorf("Watch error: %v", err)
		} else {
			c.Close()
		}
	}
}

func testVolumeWriter(t *testing.T, vol VolumeWriter, createFiles, errorFiles, mkDirs, errorDirs []string) {
	for _, fpath := range createFiles {
		w, err := vol.Create(fpath)
		if err != nil {
			t.Errorf("Create error: %v", err)
		}
		w.Write([]byte("hello"))
		w.Close()

		err = vol.Remove(fpath)
		if err != nil {
			t.Errorf("Remove error: %v", err)
		}
	}

	for _, fpath := range errorFiles {
		_, err := vol.Create(fpath)
		if _, ok := err.(*os.PathError); !ok {
			t.Errorf("Open should return pathError. path: %v err: %v", fpath, err)
		}

		_, err = vol.OpenFile(fpath, syscall.O_CREAT, 0)
		if _, ok := err.(*os.PathError); !ok {
			t.Errorf("OpenFile should return pathError. path: %v err: %v", fpath, err)
		}
	}

	for _, fpath := range errorDirs {
		err := vol.Mkdir(fpath, 644)
		if _, ok := err.(*os.PathError); !ok {
			t.Errorf("ReadDir should return pathError. path: %v err: %v", fpath, err)
		}
	}
}
