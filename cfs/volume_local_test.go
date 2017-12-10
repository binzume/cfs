package main

import (
	"fmt"
	"testing"
)

func TestLocalVolume(t *testing.T) {
	{
		vol := NewLocalVolume(".", "test", false)
		files, err := vol.ReadDir("")
		if err != nil {
			t.Errorf("error: %v", err)
		}
		fmt.Println(files)
		buf := make([]byte, 100)
		vol.Read(files[0].Name, buf, 0)
		fmt.Println(buf)
	}
}
