package application

import (
	"context"
	"sync"
)

type caseLocks struct {
	mu    sync.Mutex
	locks map[string]*caseLock
}

// caseLock serializes command execution for a single case. The holder signals
// ownership by sending a struct{}{} onto hold and releases by receiving from
// hold, allowing waiters to race the acquire against context cancellation.
type caseLock struct {
	hold chan struct{}
	refs int
}

func newCaseLocks() *caseLocks { return &caseLocks{locks: map[string]*caseLock{}} }

func (c *caseLocks) lock(ctx context.Context, caseID string) (func(), error) {
	c.mu.Lock()
	entry := c.locks[caseID]
	if entry == nil {
		entry = &caseLock{hold: make(chan struct{}, 1)}
		c.locks[caseID] = entry
	}
	entry.refs++
	c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		c.releaseReference(caseID, entry)
		return nil, err
	}
	select {
	case entry.hold <- struct{}{}:
		return func() {
			<-entry.hold
			c.releaseReference(caseID, entry)
		}, nil
	case <-ctx.Done():
		c.releaseReference(caseID, entry)
		return nil, ctx.Err()
	}
}

func (c *caseLocks) releaseReference(caseID string, entry *caseLock) {
	c.mu.Lock()
	entry.refs--
	if entry.refs == 0 {
		delete(c.locks, caseID)
	}
	c.mu.Unlock()
}
