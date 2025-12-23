package udp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sstmk-onvif/internal/events"
	"sstmk-onvif/internal/registry"
	"time"
)

// Структура сообщения от рамки
type DeviceMessage struct {
	CMD           int
	SerialNumber  string
	DeviceName    string
	ObjectName    string
	IPAddress     int
	Port          int
	ConfigVersion int
	UID           int
}

func Start(ctx context.Context, reg *registry.Store, evbuf events.Buffer) error {
	serverAddr, err := net.ResolveUDPAddr("udp", ":8989")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		return err
	}
	defer func(conn *net.UDPConn) {
		err := conn.Close()
		if err != nil {
			log.Print("[UDP] Failed to close UDP connection")
		}
	}(conn)

	// Горутина для широковещательного опроса
	go func() {
		broadcastAddr, _ := net.ResolveUDPAddr("udp", "255.255.255.255:3000")
		// Используем DialUDP для отправки
		bcConn, err := net.DialUDP("udp", nil, broadcastAddr)
		if err != nil {
			return
		}

		defer func(bcConn *net.UDPConn) {
			err := bcConn.Close()
			if err != nil {
				log.Print("[UDP] Failed to close UDP broadcast connection")
			}
		}(bcConn)

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Printf("PING")
				bcConn.Write([]byte("PING_METALD_FRAMES"))
			}
		}
	}()

	log.Printf("[UDP] Сервер мониторинга рамок запущен на :8989")

	buf := make([]byte, 2048)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue // Тайм-аут или ошибка чтения
			}

			var msg DeviceMessage
			if err := json.Unmarshal(buf[:n], &msg); err != nil {
				continue
			}

			handleMessage(msg, addr, reg, evbuf)
		}
	}
}

func handleMessage(msg DeviceMessage, addr *net.UDPAddr, reg *registry.Store, evbuf events.Buffer) {
	if msg.CMD == 0x00 {
		// Логика регистрации...
		return
	}

	// ОШИБКА БЫЛА ТУТ: нельзя просто сделать evbuf.Push("строка")
	// Нужно создать структуру Event
	ev := events.Event{
		DeviceID: msg.SerialNumber,
		Topic:    "detector/event",
		// Сериализуем данные обратно в JSON или формируем строку для Payload
		Payload: []byte(fmt.Sprintf("Object: %s, UID: %d", msg.ObjectName, msg.UID)),
		Time:    time.Now(),
	}

	// Теперь типы совпадают
	evbuf.Push(ev)
}
