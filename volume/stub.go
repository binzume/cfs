package volume

type StubVolume struct {
}

func NewStubVolume() *StubVolume {
	return &StubVolume{}
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

func (v *StubVolume) Open(path string) (reader FileReadCloser, err error) {
	return nil, Unsupported
}
