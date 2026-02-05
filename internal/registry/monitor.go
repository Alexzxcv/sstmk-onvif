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
		// Пропускаем вшитые устройства - они всегда online
		if s.isBuiltInDevice(dev.UID) {
			continue
		}

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

// isBuiltInDevice проверяет, является ли устройство вшитым
func (s *Store) isBuiltInDevice(uid string) bool {
	builtInDevices := []string{"gate-001", "gate-002", "gate-003", "gate-004"}
	for _, builtInUID := range builtInDevices {
		if uid == builtInUID {
			return true
		}
	}
	return false
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
