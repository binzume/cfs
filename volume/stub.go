package volume

import (
	"io"
	"sync"
)

type StubVolume struct {
	lock sync.Mutex
}

func NewStubVolume() *StubVolume {
	return &StubVolume{}
}

func (v *StubVolume) Locker() sync.Locker {
	return &v.lock
}

func (v *StubVolume) Available() bool {
	return true
}

func (v *StubVolume) Stat(path string) (*FileStat, error) {
	return nil, Unsupported
}

func (v *StubVolume) ReadDir(path string) ([]*FileEntry, error) {
	return nil, Unsupported
}

func (v *StubVolume) Open(path string) (reader io.ReadCloser, err error) {
	return nil, Unsupported
}
