package runtime

import (
	"sync"
)

type Counter interface {
	Count(hash string) int
}

func NewCounter() Counter {
	return &counter{
		seqs: map[string]int{},
	}
}

type counter struct {
	sync.Mutex
	seqs map[string]int
}

func (s *counter) Count(hash string) int {
	s.Lock()
	defer s.Unlock()

	count, ok := s.seqs[hash]
	if !ok {
		s.seqs[hash] = 0
		return 0
	}

	count++
	s.seqs[hash] = count

	return count
}
