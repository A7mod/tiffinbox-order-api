package kitchen

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/A7mod/tiffinbox-order-api/internal/orders"
)

var ErrKitchenFull = errors.New("kitchen full")

type Pool struct {
	queue    chan string /// the ticket rail (buffered)
	workers  int
	prepTime time.Duration
	store    *orders.Store
	log      *slog.Logger
	wg       sync.WaitGroup // counts chefs still working
}

func New(workers, queueSize int, prep time.Duration, store *orders.Store, log *slog.Logger) *Pool {
	return &Pool{
		queue: make(chan string, queueSize), workers: workers,
		prepTime: prep, store: store, log: log,
	}
}

func (p *Pool) Start() {
	for i := 1; i <= p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i) // one goroutine per check
	}
}

func (p *Pool) worker(chef int) {
	defer p.wg.Done()
	for id := range p.queue { // loops until the rail is closed AND empty
		p.store.SetStatus(id, orders.StatusPreparing)
		time.Sleep(p.prepTime) // cooking
		p.store.SetStatus(id, orders.StatusReady)
		p.log.Info("order ready", "id", id, "chef", chef)
	}
}

// ASubmit tries to pin a ticket. Waits at most 100ms, then gives up (due to backpressure)
func (p *Pool) Submit(ctx context.Context, id string) error {
	select {
	case p.queue <- id:
		return nil
	case <-ctx.Done(): //client hung up
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return ErrKitchenFull
	}
}

// Stop: no new tickets, let chefs finish what's on the rail.
func (p *Pool) Stop() {
	close(p.queue)
	p.wg.Wait()
}

func (p *Pool) Depth() int { return len(p.queue) } // tickets waiting (a metric in stage 4)
