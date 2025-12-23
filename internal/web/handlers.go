package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sstmk-onvif/internal/events"
	"sstmk-onvif/internal/state"
	"strings"
	"time"
)

// /api/v1/health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"time": time.Now().UTC().Format(time.RFC3339),
	})
}

// /api/v1/devices
func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method not allowed"})
		return
	}
	devs := s.reg.List() // берём список из registry
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": devs})
}

// /api/v1/devices/{id}/(ping|status)
func (s *Server) handleDeviceAPI(w http.ResponseWriter, r *http.Request) {
	// Expect path like: /api/v1/devices/{id}/ping or /status
	p := strings.TrimPrefix(r.URL.Path, "/api/v1/devices/")
	parts := strings.SplitN(p, "/", 2)
	if len(parts) != 2 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id, action := parts[0], parts[1]
	switch action {
	case "ping":
		s.handleDevicePing(w, r, id)
	case "status":
		s.handleDeviceStatus(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

type devicePatchRequest struct {
	Enabled *bool `json:"enabled"`
}

// /api/v1/device/{id}
func (s *Server) handleDevicePutch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": "method not allowed",
		})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/device/")
	if id == "" || id == "/api/v1/device" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": "device id is required",
		})
		return
	}

	_, ok := s.reg.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"ok":    false,
			"error": "device not found",
		})
		return
	}

	var req devicePatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": "invalid json",
		})
		return
	}

	if req.Enabled == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": "field 'enabled' is required",
		})
		return
	}

	// 1) Обновляем флаг в реестре
	s.reg.SetEnabled(id, *req.Enabled)

	// 2) (опционально) старт/стоп SSTMK-обмена
	if *req.Enabled {
		// s.sstmk.Start(id) // если понадобится
	} else {
		// s.sstmk.Stop(id)
	}

	// 3) Берём свежее состояние всех устройств и пишем в state.json
	devs := s.reg.List()
	if err := state.SaveDevices(s.statePath, devs); err != nil {
		// Не роняем запрос, но логируем
		// Можно, если хочешь, вернуть 500, но обычно достаточно лога
		log.Printf("state save error: %v", err)
	}

	// 4) Возвращаем обновлённое устройство
	dev, _ := s.reg.Get(id)

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"data": dev,
	})
}

func (s *Server) handleDevicePing(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method not allowed"})
		return
	}
	// Drain body (ack/metrics not used yet)
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r.Body)
	_ = r.Body.Close()

	// mark device online
	s.reg.SetOnline(id, true)

	// long-poll commands up to ~25s (match emulator default)
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	cmds := s.hub.LongPoll(ctx, id)

	writeJSON(w, http.StatusOK, map[string]any{
		"pong":     true,
		"ts":       time.Now().Unix(),
		"commands": cmds,
	})
}

func (s *Server) handleDeviceStatus(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "method not allowed"})
		return
	}
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	// push to events buffer
	if s.evbuf != nil && len(body) > 0 {
		s.evbuf.Push(events.Event{DeviceID: id, Topic: "status", Payload: body, Time: time.Now()})
	}
	// mark device online on status as well
	s.reg.SetOnline(id, true)

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// /api/v1/events/stream — SSE со всеми событиями из evbuf
func (s *Server) handleEventsStream(w http.ResponseWriter, r *http.Request) {
	if s.evbuf == nil {
		http.Error(w, "events buffer not enabled", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Чтобы сразу что-то отдать и пробить буфер
	_, _ = w.Write([]byte(": welcome\n\n"))
	flusher.Flush()

	// будем брать события, новее этого времени
	last := time.Now().Add(-time.Second)

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	type uiEvent struct {
		DeviceID string          `json:"device_id"`
		Topic    string          `json:"topic"`
		Time     time.Time       `json:"time"`
		Payload  json.RawMessage `json:"payload"`
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("sse: client disconnected")
			return

		case <-heartbeat.C:
			// периодический keepalive, даже если нет событий
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()

		case <-time.After(500 * time.Millisecond):
			// забираем новые события из ring-buffer
			events := s.evbuf.Pull(last, 100) // Last(after, max) — см. твой events.go
			if len(events) == 0 {
				continue
			}
			last = events[len(events)-1].Time

			for _, e := range events {
				// если нужно только наши аварии с COM порта:
				if e.Topic != "input" {
					continue
				}

				u := uiEvent{
					DeviceID: e.DeviceID,
					Topic:    e.Topic,
					Time:     e.Time,
					Payload:  e.Payload, // тут уже JSON {"input":..,"state":..}
				}

				data, err := json.Marshal(u)
				if err != nil {
					log.Printf("sse: marshal error: %v", err)
					continue
				}

				// формат SSE: "data: ...\n\n"
				_, _ = w.Write([]byte("data: "))
				_, _ = w.Write(data)
				_, _ = w.Write([]byte("\n\n"))
			}
			flusher.Flush()
		}
	}
}
