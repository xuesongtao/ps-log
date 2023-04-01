package line

// Single 单行内容
type Single struct {
	line []byte
}

func NewSing() *Single {
	return &Single{}
}

func (s *Single) reset() {
	s.line = nil
}

func (s *Single) Null() bool {
	return len(s.line) == 0
}

func (s *Single) Line() []byte {
	defer s.reset()
	return s.line
}

func (s *Single) Append(data []byte) bool {
	s.line = data
	return true
}
