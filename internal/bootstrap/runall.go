package bootstrap

import (
	"context"
	"log"
	"sync"
	"time"

	"sstmk-onvif/internal/adapters"
	"sstmk-onvif/internal/adapters/tcp"
	"sstmk-onvif/internal/config"
	"sstmk-onvif/internal/events"
	"sstmk-onvif/internal/registry"

	"sstmk-onvif/internal/discovery"
	"sstmk-onvif/internal/httpdev"
)

type sinkImpl struct{ buf events.Buffer }

func (s *sinkImpl) OnRaw(deviceID string, payload []byte) {
	s.buf.Push(events.Event{DeviceID: deviceID, Topic: "raw", Payload: payload, Time: time.Now()})
}

func RunAll(ctx context.Context, cfg *config.Config, reg *registry.Store, buf events.Buffer) error {
	// роутинг адаптеров
	factoryMap := map[string]adapters.Factory{
		"tcp": tcp.New,
	}

	// 1) Запуск мониторинга устройств
	go reg.StartMonitoring(ctx, 30*time.Second)
	log.Printf("[Bootstrap] Device monitoring started (interval: 30s)")

	// 2) поднимаем HTTP-серверы для каждого устройства
	if err := httpdev.StartAll(ctx, cfg, reg); err != nil {
		return err
	}

	// 3) WS-Discovery
	if err := discovery.Start(ctx, cfg, reg); err != nil {
		return err
	}

	// 4) Адаптеры (получение сырых данных)
	var wg sync.WaitGroup
	sink := &sinkImpl{buf: buf}
	for _, m := range reg.List() {
		f, ok := factoryMap[m.Adapter]
		if !ok {
			continue
		}
		ad, err := f(m.UID, m.AdapterDS, sink)
		if err != nil {
			log.Printf("adapter %s: %v", m.UID, err)
			continue
		}
		wg.Add(1)
		go func(a adapters.Adapter) {
			defer wg.Done()
			if err := a.Start(ctx); err != nil {
				log.Printf("adapter stopped: %v", err)
			}
		}(ad)
	}

	// ждём завершения
	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}
