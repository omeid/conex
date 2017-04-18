package conex

import (
	"sync"
)

type counter struct {
	seqs map[string]int
	lock sync.Mutex
}

func (s *counter) Count(hash string) int {
	s.lock.Lock()
	defer s.lock.Unlock()

	count, ok := s.seqs[hash]
	if !ok {
		s.seqs[hash] = 0
		return 0
	}

	count++
	s.seqs[hash] = count

	return count
}
