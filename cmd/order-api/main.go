package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)


func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))  // JSON logs: k8s-friendly
	port := getenv("PORT", "9090")
	
	var draining atomic.Bool       //flips true on SIGTERM; readiness reads it

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))       // liveness: "process is alive"
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if draining.Load() {
			http.Error(w, "draining", http.StatusServiceUnavailable)  // stop sending me  traffic
		    return
		}
		w.Write([]byte("ready"))
	})

	srv := &http.Server{Addr: ":" + port, Handler: mux}

	// ctx is cancelled when Kubernetes (or Ctrl+C) sends SIGTERM/SIGINT
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {  // server runs in its own goroutine
		log.Info("starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()    //main blocks here until a signal arrives
	log.Info("shutdown signal received, draining")
	draining.Store(true)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil { // finish in-flight requests, refuse new ones
		log.Error("shutdown error", "err", err)
	}
	log.Info("bye")
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}