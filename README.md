# [WIP] Simple FileSystem abstraction layer library for Go

[![Build Status](https://github.com/binzume/cfs/actions/workflows/test.yaml/badge.svg)](https://github.com/binzume/cfs/actions)
[![codecov](https://codecov.io/gh/binzume/cfs/branch/master/graph/badge.svg)](https://codecov.io/gh/binzume/cfs)
[![GoDoc](https://godoc.org/github.com/binzume/cfs?status.svg)](https://godoc.org/github.com/binzume/cfs)
[![license](https://img.shields.io/badge/license-MIT-4183c4.svg)](https://github.com/binzume/cfs/blob/master/LICENSE)

- Consistent API for multiple backends.
- Easy to implement backends.
- FUSE support (Windows/Linux)
- TODO: fs.FS interface

## Backend

- local storage
- memory
- zip (Readonly)
- http (Readonly)

## Usage

### local

```golang
vol := volume.NewLocalVolume("/var/contents")

// equiv: r, err := os.Open("/var/contents/hello.txt")
r, err := vol.Open("hello.txt")
```

### group

```golang
vol := volume.NewVolumeGroup()
vol.AddVolume("aaa", volume.NewLocalVolume("/var/contents_a"))
vol.AddVolume("bbb/data", volume.NewLocalVolume("/var/contents_b"))

// equiv: r, err := os.Open("/var/contents_a/hello.txt")
r, err := vol.Open("aaa/hello.txt")

// equiv: r, err := os.Open("/var/contents_b/index.html")
r, err := vol.Open("bbb/data/index.html")
```

### http

```golang
vol := httpvolume.NewHTTPVolume("", false)
r, err := vol.Open("https://example.com/hoge.txt")
```
