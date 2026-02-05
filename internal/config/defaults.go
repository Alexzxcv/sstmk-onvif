package config

import (
	"time"
)

func Defaults() *Config {

	return &Config{
		PublicIP:     "",
		LANIfName:    "br-lan",
		DevicePath:   "/onvif/device_service",
		EventsPath:   "/onvif/events",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,

		Web: WebConfig{
			Host:      "0.0.0.0",
			Port:      8080,
			StaticDir: "./webui/dist",
		},

		TTY: TTYConfig{
			Enabled:  false,
			Device:   "/dev/ttyACM0",
			BaudRate: 250000,
			DataBits: 8,
			StopBits: 1,
			Parity:   "none",
		},
	}
}
