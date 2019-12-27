package volume

import (
	"testing"
)

func TestStubVolume(t *testing.T) {
	vol := NewStubVolume()
	testVolume(t, vol,
		[]string{},
		[]string{"anyfile"},
		[]string{},
		[]string{"anydir"},
	)
}
