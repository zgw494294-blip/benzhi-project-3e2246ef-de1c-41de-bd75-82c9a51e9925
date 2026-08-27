package application

import "sync"

type caseLocks struct {
	mu    sync.Mutex
	locks map[string]*caseLock
}

type caseLock struct {
	mu   sync.Mutex
	refs int
}

func newCaseLocks() *caseLocks { return &caseLocks{locks: map[string]*caseLock{}} }

func (c *caseLocks) lock(caseID string) func() {
	c.mu.Lock()
	entry := c.locks[caseID]
	if entry == nil {
		entry = &caseLock{}
		c.locks[caseID] = entry
	}
	entry.refs++
	c.mu.Unlock()
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		c.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(c.locks, caseID)
		}
		c.mu.Unlock()
	}
}
