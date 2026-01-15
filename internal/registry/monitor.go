package registry

import (
	"context"
	"log"
	"net"
	"time"
)

// StartMonitoring запускает мониторинг устройств
func (s *Store) StartMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkDevices()
		}
	}
}

// checkDevices проверяет доступность всех устройств
func (s *Store) checkDevices() {
	devices := s.List()

	for _, dev := range devices {
		online := s.pingDevice(dev.IP)

		// Обновляем статус только если изменился
		if dev.Online != online {
			s.SetOnline(dev.UID, online)
			if online {
				log.Printf("[Monitor] Device %s (%s) is ONLINE", dev.UID, dev.IP)
			} else {
				log.Printf("[Monitor] Device %s (%s) is OFFLINE", dev.UID, dev.IP)
			}
		}
	}
}

// pingDevice проверяет доступность устройства по IP
func (s *Store) pingDevice(ip string) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", ip+":50000", timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
