# [WIP] Simple FileSystem abstraction layer library for Golang

- Consistent API for multiple backends.
- Easy to implement backends.

## Backend

- local storage
- memory
- zip

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
