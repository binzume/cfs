// +build !windows

package main

import "os"
import "syscall"

func GetATime(fi os.FileInfo) int64 {
	return fi.Sys().(*syscall.Stat_t).Mtim.Nano()
}

func GetCTime(fi os.FileInfo) int64 {
	return fi.Sys().(*syscall.Stat_t).Ctim.Nano()
}
