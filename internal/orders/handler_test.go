package orders

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeKitchen satisfies Enqueuer: the interface pays off here.
type fakeKitchen struct{ err error }

func (f fakeKitchen) Submit(ctx context.Context, id string) error { return f.err }

func newServer(k Enqueuer) (*httptest.Server, *Store) {
	store := NewStore()
	mux := http.NewServeMux()
	NewHandler(store, k).Register(mux)
	return httptest.NewServer(mux), store // a real HTTP server on a random port
}

func post(t *testing.T, url, body string) int {
	t.Helper()
	resp, err := http.Post(url+"/orders", "application/json", strings.NewReader(body))
	if err != nil {
		t.Errorf("post failed: %v", err) // Errorf, not Fatal: safe from goroutines
		return 0
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestCreateOrder(t *testing.T) {
	tests := []struct {
		name    string
		kitchen Enqueuer
		body    string
		want    int
	}{
		{"valid order", fakeKitchen{}, `{"customer":"aamod","items":["dal"]}`, http.StatusAccepted},
		{"missing items", fakeKitchen{}, `{"customer":"aamod"}`, http.StatusBadRequest},
		{"bad json", fakeKitchen{}, `{oops`, http.StatusBadRequest},
		{"kitchen full", fakeKitchen{err: errors.New("full")}, `{"customer":"aamod","items":["dal"]}`, http.StatusTooManyRequests},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newServer(tc.kitchen)
			defer srv.Close()
			if got := post(t, srv.URL, tc.body); got != tc.want {
				t.Errorf("want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestGetUnknownOrder(t *testing.T) {
	srv, _ := newServer(fakeKitchen{})
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/orders/ord-999")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestConcurrentOrdersGetUniqueIDs(t *testing.T) {
	srv, store := newServer(fakeKitchen{})
	defer srv.Close()
	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { // 50 customers ordering at the same instant
			defer wg.Done()
			post(t, srv.URL, `{"customer":"rush","items":["thali"]}`)
		}()
	}
	wg.Wait()
	store.mu.RLock()
	got := len(store.orders)
	store.mu.RUnlock()
	if got != n {
		t.Errorf("want %d orders, got %d (duplicate IDs overwrote each other?)", n, got)
	}
}
