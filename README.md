# [WIP] Simple FileSystem abstraction layer library for Golang

[![Build Status](https://travis-ci.com/binzume/cfs.svg?branch=master)](https://travis-ci.com/binzume/cfs)
[![codecov](https://codecov.io/gh/binzume/cfs/branch/master/graph/badge.svg)](https://codecov.io/gh/binzume/cfs)

- Consistent API for multiple backends.
- Easy to implement backends.

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
vol := volume.NewHTTPVolume("", false)
r, err := vol.Open("http://example.com/hoge.txt")
```
