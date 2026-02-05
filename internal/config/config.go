package config

import (
	"fmt"
	"log"
	"os"
	"sstmk-onvif/internal/registry"
	"time"

	"gopkg.in/yaml.v3"
)

// const defaultConfigPath = "./webui/config/sstmk-onvif.yml"

var defaultConfigPath string
var defaultStatePath string

type Device = registry.Device

type WebConfig struct {
	Host      string `yaml:"host"`       // 0.0.0.0
	Port      int    `yaml:"port"`       // 8080
	StaticDir string `yaml:"static_dir"` // ./webui/dist
}

type UsbConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    string `yaml:"port"` // "/dev/ttyACM0", "/dev/ttyUSB0" или "COM5"
	Baud    int    `yaml:"baud"`
}

type TTYConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Device   string `yaml:"device"`   // "/dev/ttyUSB0", "COM1"
	BaudRate int    `yaml:"baudrate"` // 9600, 115200
	DataBits int    `yaml:"databits"` // 7, 8
	StopBits int    `yaml:"stopbits"` // 1, 2
	Parity   string `yaml:"parity"`   // "none", "odd", "even"
}

type SSTMKConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"base_url"`
}

type Config struct {
	PublicIP     string        `yaml:"public_ip"`
	LANIfName    string        `yaml:"lan_if"`
	DevicePath   string        `yaml:"device_path"`
	EventsPath   string        `yaml:"events_path"`
	Devices      []Device      `yaml:"devices"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	Web          WebConfig     `yaml:"web"`
	TTY          TTYConfig     `yaml:"tty"`
	SSTMK        SSTMKConfig   `yaml:"sstmk"`
}

func Load() (*Config, error) {

	cfg := Defaults()

	// if runtime.GOOS == "linux" {
	// defaultConfigPath = "./webui/config/sstmk-onvif.yml"
	// } else {
	defaultConfigPath = "./configs/sstmk-onvif.yml"
	// }

	// if runtime.GOOS == "linux" {
	// 	defaultStatePath = ".local/share/sstmk-onvif/state.json"
	// } else {
	// 	defaultStatePath = "./state/state.json"
	// }
	defaultStatePath = "./webui/config/state.json"
	log.Printf("Default Config Path=%s", defaultConfigPath)
	// path := os.Getenv("SSTMK_CONFIG")
	path := defaultConfigPath

	wd, _ := os.Getwd()
	log.Printf("Load config: path=%s, wd=%s", path, wd)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("cannot parse yaml %s: %w", path, err)
	}

	log.Printf("config loaded, devices=%d", len(cfg.Devices))
	for i, dev := range cfg.Devices {
		log.Printf("Config Device %d: UID=%s, Name=%s, Vendor=%s, Serial=%s, Firmware=%s", i, dev.UID, dev.Name, dev.Vendor, dev.SerialNumber, dev.Version)
	}
	return cfg, nil
}
