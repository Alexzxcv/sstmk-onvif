package sstmk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type SSTMKEvent struct {
	DeviceID  string    `json:"device_id"`
	Timestamp time.Time `json:"timestamp"`
	State     uint32    `json:"state"`
	In        uint32    `json:"in"`
	Out       uint32    `json:"out"`
	Inside    uint32    `json:"inside"`
	Speed     float32   `json:"speed"`
	Level     uint32    `json:"level"`
	Metal     struct {
		Alarms    uint32 `json:"alarms"`
		AlarmsIn  uint32 `json:"alarms_in"`
		AlarmsOut uint32 `json:"alarms_out"`
	} `json:"metal"`
	Image string `json:"image"` // Base64 encoded image
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) SendEvent(event *SSTMKEvent) error {
	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/api/events",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to send event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	log.Printf("[SSTMK] Event sent successfully for device %s", event.DeviceID)
	return nil
}
