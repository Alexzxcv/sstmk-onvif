package sstmk

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"sstmk-onvif/internal/events"
	"sstmk-onvif/internal/onvif"
)

type Adapter struct {
	eventService *onvif.EventService
}

func NewAdapter(baseURL string) *Adapter {
	return &Adapter{
		eventService: onvif.NewEventService(baseURL),
	}
}

func (a *Adapter) ProcessEvent(event events.Event) error {
	if event.Topic != "detector/event" {
		return nil
	}

	var payloadMap map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payloadMap); err != nil {
		return err
	}

	image, _ := payloadMap["image"].(string)
	a.eventService.PublishEvent(event.DeviceID, image, "default")
	return nil
}

func (a *Adapter) Start(ctx context.Context, evbuf events.Buffer) {
	go func() {
		lastCheck := time.Now()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				events := evbuf.Pull(lastCheck, 10)
				for _, event := range events {
					if err := a.ProcessEvent(event); err != nil {
						log.Printf("[SSTMK] Error processing event: %v", err)
					}
				}
				if len(events) > 0 {
					lastCheck = events[len(events)-1].Time
				}
			}
		}
	}()
}

func (a *Adapter) GetEventService() *onvif.EventService {
	return a.eventService
}
