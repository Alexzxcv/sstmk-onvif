package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"sstmk-onvif/internal/config"
	"sstmk-onvif/internal/events"

	"sstmk-onvif/internal/hub"
	"sstmk-onvif/internal/registry"
)

type Server struct {
	http      *http.Server
	cfg       config.WebConfig
	reg       *registry.Store
	evbuf     events.Buffer // 👈 теперь интерфейс из твоего пакета
	hub       *hub.Hub
	statePath string // хранение данных
}

func New(cfg config.WebConfig, reg *registry.Store, evbuf events.Buffer, hub *hub.Hub, statePath string) *Server {
	mux := http.NewServeMux()

	// --- SSE событий из ring buffer ---
	// Пуллим события чаще (например, раз в 500мс) и шлём клиенту
	s := &Server{
		cfg:       cfg,
		reg:       reg,
		evbuf:     evbuf,
		hub:       hub,
		statePath: statePath,
	}

	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/devices", s.handleDevices)
	mux.HandleFunc("/api/v1/devices/", s.handleDeviceAPI)
	mux.HandleFunc("/api/v1/device/", s.handleDevicePutch)
	mux.HandleFunc("/api/v1/events/stream", s.handleEventsStream)

	// --- STATIC ---
	staticDir := filepath.Clean(cfg.StaticDir)
	useStatic := staticDir != "" && dirExists(staticDir) && fileExists(filepath.Join(staticDir, "index.html"))
	if useStatic {
		fs := http.FileServer(http.Dir(staticDir))
		mux.Handle("/static/", http.StripPrefix("/static/", fs))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		})
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"message":    "web UI not bundled (static_dir missing or index.html not found)",
				"static_dir": staticDir,
			})
		})
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	s.http = &http.Server{
		Addr:              addr,
		Handler:           withCommonHeaders(mux),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		// WriteTimeout:      30 * time.Second,
		IdleTimeout: 60 * time.Second,
	}
	return s
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		log.Printf("[web] listening on http://%s", s.http.Addr)
		if err := s.http.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.http.Shutdown(shCtx); err != nil {
			log.Printf("[web] shutdown error: %v", err)
		} else {
			log.Printf("[web] stopped")
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func withCommonHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS заголовки
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "600")
		// Preflight-запросы: просто отвечаем 204 и выходим
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func dirExists(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}
