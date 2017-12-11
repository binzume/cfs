package volume

import (
	"testing"
)

func TestVolume(t *testing.T) {
	{
		vol1 := NewLocalVolume(".", "test1", false)
		vol2 := NewLocalVolume("..", "test2", false)
		vol := NewVolumeGroup()
		vol.Add("hoge", vol1)
		vol.Add("hoge1/hoge2", vol2)

		st, err := vol.Stat("")
		if err != nil {
			t.Errorf("error: %v", err)
		}
		if !st.IsDir {
			t.Errorf("stat err: %v", st)
		}

		st, err = vol.Stat("hoge1/hoge2")
		if err != nil {
			t.Errorf("error: %v", err)
		}
		if !st.IsDir {
			t.Errorf("stat err: %v", st)
		}
	}
}
