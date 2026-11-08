package gc

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/alexandrmotologa/uploadgate/internal/db"
	"github.com/alexandrmotologa/uploadgate/internal/storage"
)

// Cleaner runs periodic sweeps to prune expired uploads and abandoned presigned sessions.
type Cleaner struct {
	repo             *db.Repository
	storageMgr       *storage.Manager
	interval         time.Duration
	abandonedTimeout time.Duration
	ctx              context.Context
	cancelFunc       context.CancelFunc
	wg               sync.WaitGroup
}

// NewCleaner creates a new GC cleaner worker.
func NewCleaner(repo *db.Repository, storageMgr *storage.Manager, interval, abandonedTimeout time.Duration) *Cleaner {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if abandonedTimeout <= 0 {
		abandonedTimeout = 2 * time.Hour
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Cleaner{
		repo:             repo,
		storageMgr:       storageMgr,
		interval:         interval,
		abandonedTimeout: abandonedTimeout,
		ctx:              ctx,
		cancelFunc:       cancel,
	}
}

// Start launches the cleaner loop in a background goroutine.
func (c *Cleaner) Start() {
	c.wg.Add(1)
	go c.runLoop()
}

// Stop terminates the cleaner gracefully.
func (c *Cleaner) Stop() {
	c.cancelFunc()
	c.wg.Wait()
}

func (c *Cleaner) runLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Run initial sweep on startup
	c.sweep()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.sweep()
		}
	}
}

func (c *Cleaner) sweep() {
	now := time.Now().UTC()
	abandonedCutoff := now.Add(-c.abandonedTimeout)

	expired, err := c.repo.FindExpiredUploads(c.ctx, abandonedCutoff, now)
	if err != nil {
		log.Printf("[GC Cleaner] Failed to query expired uploads: %v", err)
		return
	}

	if len(expired) == 0 {
		return
	}

	log.Printf("[GC Cleaner] Pruning %d expired or abandoned upload records...", len(expired))

	for _, u := range expired {
		prov, err := c.storageMgr.Get(u.StorageProvider)
		if err == nil {
			// Delete main asset file from storage
			_ = prov.DeleteObject(c.ctx, u.StorageKey)

			// Delete generated thumbnails from storage
			for _, th := range u.Thumbnails {
				_ = prov.DeleteObject(c.ctx, th.StorageKey)
			}
		}

		// Delete from SQLite
		if err := c.repo.DeleteUpload(c.ctx, u.ID); err != nil {
			log.Printf("[GC Cleaner] Failed to delete upload %s from database: %v", u.ID, err)
		}
	}
}
