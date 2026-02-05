package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"sstmk-onvif/internal/adapters/udp"
	"sstmk-onvif/internal/tty"

	"sstmk-onvif/internal/bootstrap"
	"sstmk-onvif/internal/config"
	"sstmk-onvif/internal/events"
	"sstmk-onvif/internal/hub"
	"sstmk-onvif/internal/registry"
	"sstmk-onvif/internal/sstmk"
	"sstmk-onvif/internal/state"
	"sstmk-onvif/internal/web"
)

// isBuiltInDevice проверяет, является ли устройство вшитым
func isBuiltInDevice(uid string) bool {
	builtInDevices := []string{"gate-001", "gate-002", "gate-003", "gate-004"}
	for _, builtInUID := range builtInDevices {
		if uid == builtInUID {
			return true
		}
	}
	return false
}

func main() {

	statePath := "./webui/config/state.json"

	log.Printf("start Load config")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// 1. Загружаем или инициализируем state.json
	st, err := state.LoadOrInit(statePath, cfg.Devices)
	if err != nil {
		log.Fatalf("state: %v", err)
	}

	// 2. Создаём registry.Store и наполняем устройствами из state
	reg := registry.NewStore()
	for _, d := range st.Devices {
		reg.Upsert(registry.Device{
			UID:          d.UID,
			SerialNumber: d.SerialNumber,
			Name:         d.Name,
			Vendor:       d.Vendor,
			Object:       d.Object,
			IP:           d.IP,
			Port:         d.Port,
			Version:      d.Version,
			Model:        d.Model,
			Revision:     d.Revision,
			Adapter:      d.Adapter,
			AdapterDS:    d.AdapterDS,
		})
		// Восстанавливаем enabled из state.json
		reg.SetEnabled(d.UID, d.Enabled)
		// Для вшитых устройств всегда online=true
		if isBuiltInDevice(d.UID) {
			reg.SetOnline(d.UID, true)
		} else {
			reg.SetOnline(d.UID, d.Online)
		}
	}

	evbuf := events.NewRing(1024)
	hb := hub.New()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	log.Printf("[cfg] web host=%q port=%d static=%q", cfg.Web.Host, cfg.Web.Port, cfg.Web.StaticDir)

	errCh := make(chan error, 2)

	// 3. Стартуем веб-сервер, передаём statePath
	sstmkAdapter := sstmk.NewAdapter(cfg.SSTMK.BaseURL)
	webSrv := web.New(cfg.Web, reg, evbuf, hb, statePath, sstmkAdapter.GetEventService())
	go func() {
		if err := webSrv.Start(ctx); err != nil {
			errCh <- err
		}
	}()

	// 4. Стартуем остальные подсистемы
	go func() {
		if err := bootstrap.RunAll(ctx, cfg, reg, evbuf); err != nil {
			errCh <- err
		}
	}()

	go func() {
		if err := udp.Start(ctx, reg, evbuf); err != nil {
			log.Printf("UDP server error: %v", err)
		}
	}()

	go func() {
		if !cfg.TTY.Enabled {
			log.Printf("[tty] disabled")
			return
		}
		ttyCfg := tty.Config{
			Device:   cfg.TTY.Device,
			BaudRate: cfg.TTY.BaudRate,
			DataBits: cfg.TTY.DataBits,
			StopBits: cfg.TTY.StopBits,
			Parity:   cfg.TTY.Parity,
		}
		if err := tty.Start(ctx, ttyCfg, evbuf); err != nil {
			// не падаем весь сервис, просто логируем
			log.Printf("tty reader stopped: %v", err)
		}
	}()

	// 5. SSTMK адаптер
	go func() {
		if !cfg.SSTMK.Enabled {
			log.Printf("[SSTMK] disabled")
			return
		}
		sstmkAdapter.Start(ctx, evbuf)
		log.Printf("[SSTMK] adapter started, base_url=%s", cfg.SSTMK.BaseURL)
	}()

	// 6. Ждём либо ошибку, либо сигнал завершения
	select {
	case err := <-errCh:
		log.Fatalf("fatal: %v", err)
	case <-ctx.Done():
		// graceful shutdown внутри Start/RunAll
	}
}
