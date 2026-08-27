package application

import (
	"context"
	"sync"
)

type caseLocks struct {
	mu    sync.Mutex
	locks map[string]*caseLock
}

type caseLock struct {
	mu   sync.Mutex
	refs int
}

func newCaseLocks() *caseLocks { return &caseLocks{locks: map[string]*caseLock{}} }

func (c *caseLocks) lock(ctx context.Context, caseID string) (func(), error) {
	c.mu.Lock()
	entry := c.locks[caseID]
	if entry == nil {
		entry = &caseLock{}
		c.locks[caseID] = entry
	}
	entry.refs++
	c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		c.releaseReference(caseID, entry)
		return nil, err
	}
	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		c.releaseReference(caseID, entry)
	}, nil
}

func (c *caseLocks) releaseReference(caseID string, entry *caseLock) {
	c.mu.Lock()
	entry.refs--
	if entry.refs == 0 {
		delete(c.locks, caseID)
	}
	c.mu.Unlock()
}
