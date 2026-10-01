package orders

import (
	"sync"
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusPreparing Status = "preparing"
	StatusReady     Status = "ready"
)

type Order struct {
	ID        string    `json:"id"`
	Customer  string    `json:"customer"`
	Items     []string  `json:"items"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Store = the order book. Many readers GET, few writers -> RWMutex.
type Store struct {
	mu     sync.RWMutex
	orders map[string]*Order
}

func NewStore() *Store { return &Store{orders: map[string]*Order{}} }

func (s *Store) Add(o Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[o.ID] = &o
}

func (s *Store) Get(id string) (Order, bool) {
	s.mu.RLock() //shared lock: many GETs at once are fine
	defer s.mu.RUnlock()
	o, ok := s.orders[id]
	if !ok {
		return Order{}, false
	}
	return *o, true // return a copy, not the pointer
}

func (s *Store) SetStatus(id string, st Status) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o, ok := s.orders[id]; ok {
		o.Status = st
	}
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.orders, id)
}
