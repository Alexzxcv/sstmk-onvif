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

type BinaryEventPacket struct {
	Cmd    uint8   // 0x05
	TS     uint32  // Timestamp
	Status [1]byte // Вложенная структура статуса
	Zones  [1]byte // Вложенная структура зон
}

type DetectorStatus struct {
	State          uint32
	In             uint32
	Out            uint32
	Inside         uint32
	Speed          float32
	CalibTimout    uint32
	Level          uint32
	Lights         uint32
	Classification struct {
		DetectorMetalType  uint32
		DetectorMetalClass uint32
		DetectorObjectType uint32
	}
	Metal struct {
		alarms     uint32
		alarms_in  uint32
		alarms_out uint32
	}
}

type ZonesConfig struct {
	Zones_h uint8
	Zones_v uint8
	Total   uint8
}

type DetectorZones struct {
	Config ZonesConfig
	Alarm  uint32
	Level  uint8
	Cnt    uint32
}

func Start(ctx context.Context, reg *registry.Store, evbuf events.Buffer) error {
	// 1. Создаем адрес для прослушивания
	serverAddr, err := net.ResolveUDPAddr("udp", ":50000")
	if err != nil {
		return err
	}

	// 2. Биндим порт 50000. Этот conn будет и слушать, и отправлять.
	conn, err := net.ListenUDP("udp", serverAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Адрес для отправки Broadcast
	broadcastAddr, _ := net.ResolveUDPAddr("udp", "255.255.255.255:50000")

	// Запускаем горутину для отправки (Discovery)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 3. ВАЖНО: Используем ТОТ ЖЕ conn для отправки.
				// Метод WriteToUDP позволяет указать адрес назначения.
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
			// Чтение данных
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				continue
			}

			// 1) Отфильтровать свой запрос Discovery
			if n == 1 && buf[0] == BP_CMD_DISCOVERY {
				// Это наш же broadcast-запрос — пропускаем
				continue
			}

			// Обработка сообщения
			// Важно копировать данные, если handleMessage работает асинхронно,
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
		// handleEvent(data, addr, reg, evbuf)
	}
}

func handleRegistration(data []byte, addr *net.UDPAddr, reg *registry.Store, evbuf events.Buffer) {
	var msg BinaryDiscoveryPacket
	reader := bytes.NewReader(data)
	// log.Printf("%v", reader)
	if err := binary.Read(reader, binary.LittleEndian, &msg); err != nil {
		log.Printf("Ошибка парсинга discovery: %v", err)
		return
	}

	cleanStr := func(b []byte) string {
		return string(bytes.TrimRight(b, "\x00"))
	}

	serial := cleanStr(msg.SN[:])

	uid := cleanStr(msg.UID[:])

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

// func handleEvent(data []byte, addr *net.UDPAddr, reg *registry.Store, evbuf events.Buffer) {
// 	var msg BinaryEventPacket
// 	reader := bytes.NewReader(data)
//
// 	// Читаем бинарные данные
// 	if err := binary.Read(reader, binary.LittleEndian, &msg); err != nil {
// 		log.Printf("Ошибка парсинга event: %v", err)
// 		return
// 	}
//
// 	// Создаем структуру для красивого вывода (DebugDTO)
// 	// Поля ОБЯЗАТЕЛЬНО с Большой Буквы для JSON
// 	type DebugEvent struct {
// 		Cmd    uint8
// 		TS     uint32
// 		Status string
// 		Zones  string
// 	}
//
// 	// Заполняем данными как есть (без преобразования в string)
// 	debugData := DebugEvent{
// 		Cmd:    msg.Cmd,
// 		TS:     msg.TS,
// 		Status: DetectorStatus,
// 		Zones:  DetectorZones,
// 	}
//
// 	// Маршалим в JSON
// 	jsonBytes, err := json.MarshalIndent(debugData, "", "  ")
// 	if err != nil {
// 		log.Printf("JSON error: %v", err)
// 		return
// 	}
//
// 	fmt.Println(string(jsonBytes))
//
// 	// ... Дальше ваша логика обработки события ...
// }
