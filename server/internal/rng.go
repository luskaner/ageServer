package internal

import (
	"math/rand/v2"
	"sync"

	"github.com/luskaner/ageServer/common/uuid"
)

var rng RandReader

type RandReader struct {
	*rand.Rand
	mu sync.Mutex
}

func (rr *RandReader) Read(p []byte) (int, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	for i := range p {
		p[i] = byte(rr.N(256))
	}
	return len(p), nil
}

func WithRng(action func(rand *RandReader)) {
	rng.WithRng(action)
}

func (rr *RandReader) WithRng(action func(rand *RandReader)) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	action(&rng)
}

func InitializeRng(seed uint64) {
	rng.Rand = rand.New(rand.NewPCG(seed, seed))
	uuid.SetRand(&rng)
}
