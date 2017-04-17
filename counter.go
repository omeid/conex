package conex

import (
	"strings"
	"sync"
)

type counter struct {
	seqs map[string]int
	lock sync.Mutex
}

func (s *counter) Count(name string, keys []string) int {
	s.lock.Lock()
	defer s.lock.Unlock()

	hash := name + "-" + strings.Join(keys, "-")

	count, ok := s.seqs[hash]
	if !ok {
		s.seqs[hash] = 0
		return 0
	}

	count++
	s.seqs[hash] = count

	return count
}
