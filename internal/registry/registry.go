package registry

import "sync"

type Device struct {
	ID        string `yaml:"id"          json:"id"`
	Name      string `yaml:"name"        json:"name"`
	Vendor    string `yaml:"vendor"      json:"vendor"`
	Model     string `yaml:"model"       json:"model"`
	Firmware  string `yaml:"firmware"    json:"firmware"`
	Serial    string `yaml:"serial"      json:"serial"`
	Hardware  string `yaml:"hardware"    json:"hardware"`
	Location  string `yaml:"location"    json:"location"`
	TypeScope string `yaml:"type_scope"  json:"type_scope"`
	Port      int    `yaml:"port"        json:"port"`
	Adapter   string `yaml:"adapter"     json:"adapter"`
	AdapterDS string `yaml:"adapterDS"   json:"adapter_ds"`

	Enabled bool `yaml:"enabled,omitempty" json:"enabled"`
	Online  bool `yaml:"-"                 json:"online"`
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
	s.data[m.ID] = m
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
