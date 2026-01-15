package udp

import (
	"context"
	"log"
	"net"
	"time"

	"sstmk-onvif/internal/events"
	"sstmk-onvif/internal/registry"
)

func Start(ctx context.Context, reg *registry.Store, evbuf events.Buffer) error {

	serverAddr, err := net.ResolveUDPAddr("udp", ":50000")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	broadcastAddr, _ := net.ResolveUDPAddr("udp", "255.255.255.255:50000")

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := conn.WriteToUDP([]byte{BP_CMD_DISCOVERY}, broadcastAddr)
				if err != nil {
					log.Printf("Ошибка отправки discovery: %v", err)
				}
			}
		}
	}()

	log.Printf("[UDP] Сервер мониторинга запущен на :50000")

	buf := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}

			// собственное сообщение BP_CMD_DISCOVERY
			if n == 1 && buf[0] == BP_CMD_DISCOVERY {
				continue
			}
			// Обработка сообщения
			// важно копировать данные, если handleMessage работает асинхронно,
			// но если синхронно - можно передавать срез буфера.
			handleMessage(buf[:n], addr, reg, evbuf)
		}
	}
}

func handleMessage(data []byte, addr *net.UDPAddr, reg *registry.Store, evbuf events.Buffer) {
	if len(data) == 0 {
		return
	}

	cmd := data[0]
	switch cmd {
	case BP_CMD_DISCOVERY:
		handleRegistration(data, addr, reg, evbuf)
	case BP_CMD_EVENT_NOTIFICATION:
		handleEvent(data, addr, reg, evbuf)
	}
}
