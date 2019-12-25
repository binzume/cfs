package volume

type StubVolume struct {
}

func NewStubVolume() *StubVolume {
	return &StubVolume{}
}

func (v *StubVolume) Available() bool {
	return true
}

func (v *StubVolume) Stat(path string) (*FileInfo, error) {
	return nil, Unsupported
}

func (v *StubVolume) ReadDir(path string) ([]*FileInfo, error) {
	return nil, Unsupported
}

func (v *StubVolume) Open(path string) (reader FileReadCloser, err error) {
	return nil, Unsupported
}
