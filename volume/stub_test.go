package volume

import (
	"os"
	"testing"
)

func TestStubVolume(t *testing.T) {
	var vol Volume = NewStubVolume()

	if !vol.Available() {
		t.Fatal("volume is unavailable")
	}

	_, err := vol.ReadDir("")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	_, err = vol.Stat("")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}

	_, err = vol.Open("")
	if _, ok := err.(*os.PathError); !ok {
		t.Errorf("should return pathError. err: %v", err)
	}
}
