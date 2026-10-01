package kitchen

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/A7mod/tiffinbox-order-api/internal/orders"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestSubmitReturnsFullWhenQueueIsFull(t *testing.T) {
	p := New(1, 2, time.Millisecond, orders.NewStore(), quietLog())
	// No Start(): no chefs, so the rail fills up and stays full.
	for i := 0; i < 2; i++ {
		if err := p.Submit(context.Background(), "x"); err != nil {
			t.Fatalf("submit %d: unexpected error %v", i, err)
		}
	}
	if err := p.Submit(context.Background(), "x"); !errors.Is(err, ErrKitchenFull) {
		t.Fatalf("want ErrKitchenFull, got %v", err)
	}
}

func TestStopDrainsAllOrders(t *testing.T) {
	store := orders.NewStore()
	p := New(2, 10, 20*time.Millisecond, store, quietLog())
	p.Start()
	ids := []string{"a", "b", "c", "d", "e"}
	for _, id := range ids {
		store.Add(orders.Order{ID: id, Status: orders.StatusQueued})
		if err := p.Submit(context.Background(), id); err != nil {
			t.Fatal(err)
		}
	}
	p.Stop() // must block until every order is cooked
	for _, id := range ids {
		if o, _ := store.Get(id); o.Status != orders.StatusReady {
			t.Errorf("order %s: want ready, got %s", id, o.Status)
		}
	}
}
