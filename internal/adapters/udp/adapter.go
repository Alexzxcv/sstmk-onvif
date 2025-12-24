package udp

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"sstmk-onvif/internal/events"
	"sstmk-onvif/internal/registry"
)

// Команды согласно docs/binary_api.md
const (
	BP_CMD_DISCOVERY          uint8 = 0x00 // Discovery Request
	BP_CMD_EVENT_NOTIFICATION uint8 = 0x05 // Уведомление о событии
	BP_CMD_ACK                uint8 = 0xFF
)

type BinaryDiscoveryPacket struct {
	Cmd      uint8
	SN       [32]byte
	Name     [64]byte
	Object   [64]byte
	IP       [4]byte
	Port     uint16
	UID      uint32
	Version  [10]byte
	GitHash  [10]byte
	Revision [10]byte
	Vendor   [32]byte
	Model    [32]byte
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
	defer conn.Close()

	go func() {
		broadcastAddr, _ := net.ResolveUDPAddr("udp", "255.255.255.255:3000")
		bcConn, err := net.DialUDP("udp", nil, broadcastAddr)
		if err != nil {
			return
		}
		defer bcConn.Close()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				bcConn.Write([]byte{BP_CMD_DISCOVERY})
			}
		}
	}()

	log.Printf("[UDP] Сервер мониторинга запущен на :8989")

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

			// ПЕРЕДАЕМ СРЕЗ БАЙТ
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
		log.Printf("[UDP] Получено событие от %s", addr)
		// Здесь должна быть логика парсинга BinaryZonesPacket
	}
}

func handleRegistration(data []byte, addr *net.UDPAddr, reg *registry.Store, evbuf events.Buffer) {
	var msg BinaryDiscoveryPacket
	reader := bytes.NewReader(data)
	if err := binary.Read(reader, binary.LittleEndian, &msg); err != nil {
		log.Printf("Ошибка парсинга discovery: %v", err)
		return
	}

	cleanStr := func(b []byte) string {
		return string(bytes.TrimRight(b, "\x00"))
	}

	serial := cleanStr(msg.SN[:])

	dev := registry.Device{
		ID:        serial,
		Name:      cleanStr(msg.Name[:]),
		Vendor:    cleanStr(msg.Vendor[:]),
		Model:     cleanStr(msg.Model[:]),
		Firmware:  cleanStr(msg.Version[:]),
		Serial:    serial,
		Hardware:  cleanStr(msg.Revision[:]),
		Location:  cleanStr(msg.Object[:]),
		TypeScope: "MetalDetector",
		AdapterDS: fmt.Sprintf("%s:%d", addr.IP.String(), msg.Port),
		Adapter:   "udp",
		Enabled:   true,
		Online:    true,
	}
	test, _ := json.MarshalIndent(dev, "", "  ")
	fmt.Println(string(test))
	reg.Upsert(dev)

	evbuf.Push(events.Event{
		DeviceID: dev.ID,
		Topic:    "system/discovery",
		Payload:  []byte(fmt.Sprintf("Device %s discovered at %s", dev.ID, addr.IP.String())),
		Time:     time.Now(),
	})
}
