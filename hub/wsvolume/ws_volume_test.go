package wsvolume

import (
	"testing"

	"github.com/binzume/cfs/volume"
)

func TestWsVolume(t *testing.T) {
	vol := NewRemoteVolume("hoge", nil)
	var _ volume.Volume = vol
	var _ volume.VolumeWriter = vol
}
