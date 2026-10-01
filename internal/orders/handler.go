package orders

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Enqueuer: anything that can accept an order. The kitchen satisfies it.
// Using an interface here avoids an import cycle (kitchen already imports orders).
type Enqueuer interface {
	Submit(ctx context.Context, id string) error
}

type Handler struct {
	store   *Store
	kitchen Enqueuer
	seq     atomic.Int64 //order number counter, safe across goroutines.
}

func NewHandler(s *Store, k Enqueuer) *Handler { return &Handler{store: s, kitchen: k} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", h.create)
	mux.HandleFunc("GET /orders/{id}", h.get)
}

type createReq struct {
	Customer string   `json:"customer"`
	Items    []string `json:"items"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Customer == "" || len(req.Items) == 0 {
		http.Error(w, "need customer and items", http.StatusBadRequest)
		return
	}
	o := Order{
		ID: fmt.Sprintf("ord-%d", h.seq.Add(1)), Customer: req.Customer,
		Items: req.Items, Status: StatusQueued, CreatedAt: time.Now(),
	}
	h.store.Add(o)
	if err := h.kitchen.Submit(r.Context(), o.ID); err != nil {
		h.store.Delete(o.ID)
		http.Error(w, "kitchen is full, try again", http.StatusTooManyRequests)
		return
	}
	writeJSON(w, http.StatusAccepted, o) // 202: accepted, not finished
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	o, ok := h.store.Get(r.PathValue("id"))
	if !ok {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
