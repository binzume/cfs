package volume

import "os"
import "syscall"

func GetATime(fi os.FileInfo) int64 {
	return fi.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds()
}

func GetCTime(fi os.FileInfo) int64 {
	return fi.Sys().(*syscall.Win32FileAttributeData).LastAccessTime.Nanoseconds()
}
