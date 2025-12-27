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

const (
	N_COILS_PER_SIDE = 6
	N_COIL_SIDES     = 2
)

type ClassificationResult struct {
	Type   uint32
	Class  uint32
	Object uint32
}

type DetectorStatus struct {
	State          uint32
	In             uint32
	Out            uint32
	Inside         uint32
	Speed          float32
	CalibTimeout   uint32
	Level          uint32
	Lights         uint32
	Classification ClassificationResult
	Metal          struct {
		Alarms    uint32
		AlarmsIn  uint32
		AlarmsOut uint32
	}
}

type ZoneConfig struct {
	ZonesH uint32
	ZonesV uint32
	Total  uint32
}

type DetectorZones struct {
	Config ZoneConfig
	Alarm  [N_COILS_PER_SIDE][N_COIL_SIDES]ClassificationResult
	Level  [N_COILS_PER_SIDE][N_COIL_SIDES]uint8
	Cnt    [N_COILS_PER_SIDE][N_COIL_SIDES]uint32
}

type BinaryEventPacket struct {
	Cmd    uint8
	TS     uint32
	Status DetectorStatus
	Zones  DetectorZones
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
		handleEvent(data, addr, reg, evbuf)
	}
}

func handleRegistration(data []byte, addr *net.UDPAddr, reg *registry.Store, evbuf events.Buffer) {
	var msg BinaryDiscoveryPacket
	reader := bytes.NewReader(data)
	// log.Printf("%v", reader)
	// log.Printf("%v", addr)
	if err := binary.Read(reader, binary.LittleEndian, &msg); err != nil {
		log.Printf("Ошибка парсинга discovery: %v", err)
		return
	}

	cleanStr := func(b []byte) string {
		return string(bytes.TrimRight(b, "\x00"))
	}

	// reg.RegisterOrUpdate()

	dev := registry.Device{
		UID:          fmt.Sprintf("%d", msg.UID),
		SerialNumber: cleanStr(msg.SN[:]),
		Name:         cleanStr(msg.Name[:]),
		Vendor:       cleanStr(msg.Vendor[:]),
		Object:       cleanStr(msg.Object[:]),
		IP:           net.IP(msg.IP[:]).String(),
		Port:         fmt.Sprintf("%d", msg.Port),
		Version:      cleanStr(msg.Version[:]),
		Model:        cleanStr(msg.Model[:]),
		Revision:     cleanStr(msg.Revision[:]),
		Adapter:      "udp",
		AdapterDS:    fmt.Sprintf("%s:%d", addr.IP.String(), msg.Port),
		Enabled:      true,
		Online:       true,
	}

	test, _ := json.MarshalIndent(dev, "", "  ")
	fmt.Println(string(test))
	reg.Upsert(dev)

	evbuf.Push(events.Event{
		DeviceID: dev.UID,
		Topic:    "system/discovery",
		Payload:  []byte(fmt.Sprintf("Device %s discovered at %s", dev.UID, addr.IP.String())),
		Time:     time.Now(),
	})

}

func handleEvent(data []byte, addr *net.UDPAddr, reg *registry.Store, evbuf events.Buffer) {
	log.Printf("%x", data)
	r := bytes.NewReader(data)

	var msg BinaryEventPacket

	binary.Read(r, binary.LittleEndian, &msg.Cmd)
	binary.Read(r, binary.LittleEndian, &msg.TS)
	binary.Read(r, binary.LittleEndian, &msg.Status.State)
	binary.Read(r, binary.LittleEndian, &msg.Status.In)
	binary.Read(r, binary.LittleEndian, &msg.Status.Out)
	binary.Read(r, binary.LittleEndian, &msg.Status.Inside)
	binary.Read(r, binary.LittleEndian, &msg.Status.Speed)
	binary.Read(r, binary.LittleEndian, &msg.Status.CalibTimeout)
	binary.Read(r, binary.LittleEndian, &msg.Status.Level)

	binary.Read(r, binary.LittleEndian, &msg.Status.Lights)
	binary.Read(r, binary.LittleEndian, &msg.Status.Classification)
	binary.Read(r, binary.LittleEndian, &msg.Status.Metal)
	binary.Read(r, binary.LittleEndian, &msg.Zones.Config)
	binary.Read(r, binary.LittleEndian, &msg.Zones.Alarm)
	binary.Read(r, binary.LittleEndian, &msg.Zones.Level)
	binary.Read(r, binary.LittleEndian, &msg.Zones.Cnt)

	// ---
	jsonBytes, _ := json.MarshalIndent(msg, "", "  ")
	fmt.Println(string(jsonBytes))
	// ---

}
