package registry

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

type Device struct {
	UID          string `yaml:"uid"          json:"uid"`
	SerialNumber string `yaml:"serialNumber" json:"serialNumber"`
	Name         string `yaml:"name"         json:"name"`
	Vendor       string `yaml:"vendor"       json:"vendor"`
	Object       string `yaml:"object"       json:"object"` // location
	IP           string `yaml:"ip"           json:"ip"`
	Port         string `yaml:"port"         json:"port"`
	Version      string `yaml:"version"      json:"version"` // fw
	Model        string `yaml:"model"        json:"model"`
	Revision     string `yaml:"revision"     json:"revision"` // hw
	Adapter      string `yaml:"adapter"      json:"adapter"`
	AdapterDS    string `yaml:"adapterDS"    json:"adapter_ds"`
	Enabled      bool   `yaml:"enabled"      json:"enabled"`
	Online       bool   `yaml:"-"            json:"online"`
}

type Store struct {
	mu   sync.RWMutex
	data map[string]Device
}

func NewStore() *Store {
	return &Store{data: map[string]Device{}}
}

func (s *Store) Get(id string) (Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[id]
	return v, ok
}

func (s *Store) Upsert(m Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[m.UID] = m
}

func (s *Store) Update(m Device) {
	s.Upsert(m)
}

func (s *Store) SetOnline(id string, online bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[id]
	if !ok {
		return
	}
	v.Online = online
	s.data[id] = v
}

func (s *Store) SetEnabled(id string, enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[id]
	if !ok {
		return
	}
	v.Enabled = enabled
	s.data[id] = v
}

func (s *Store) List() []Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Device, 0, len(s.data))
	for _, v := range s.data {
		out = append(out, v)
	}
	return out
}

// ВАЖНО ↓
// Только эти показываем в ONVIF / discovery
func (s *Store) ListVisible() []Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Device, 0)
	for _, v := range s.data {
		if v.Enabled {
			out = append(out, v)
		}
	}
	return out
}

func (s *Store) RegisterOrUpdate() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// хотим вывести все девайсы в виде JSON-массива
	devices := make([]Device, 0, len(s.data))
	for _, d := range s.data {
		devices = append(devices, d)
	}

	b, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		log.Printf("store json marshal error: %v", err)
		return false
	}

	fmt.Println(string(b))
	return true
}
