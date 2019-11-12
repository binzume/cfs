package ws_volume

import (
	"testing"

	"../volume"
)

func TestWsVolume(t *testing.T) {
	vol := NewRemoteVolume("hoge", nil)
	var _ volume.Volume = vol
	var _ volume.VolumeWriter = vol
}
