package udp

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"path/filepath"
	"time"

	"sstmk-onvif/internal/events"
	"sstmk-onvif/internal/registry"
)

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

	dev := registry.Device{
		UID:          fmt.Sprintf("%d", msg.UID),
		SerialNumber: cleanStr(msg.SN[:]),
		Name:         cleanStr(msg.Name[:]),
		Vendor:       cleanStr(msg.Vendor[:]),
		Object:       cleanStr(msg.Object[:]),
		IP:           net.IP(msg.IP[:]).String(),
		// Port будет назначен автоматически в Upsert
		Version:   cleanStr(msg.Version[:]),
		Model:     cleanStr(msg.Model[:]),
		Revision:  cleanStr(msg.Revision[:]),
		Adapter:   "udp",
		AdapterDS: fmt.Sprintf("%s:%d", addr.IP.String(), msg.Port),
		Enabled:   true,
		Online:    true,
	}

	reg.Upsert(dev)

	// Получаем обновленное устройство с назначенным портом
	if updated, ok := reg.Get(dev.UID); ok {
		log.Printf("[UDP] Device %s registered with port %s", updated.UID, updated.Port)
		// jsonData, err := json.MarshalIndent(updated, "", "    ")
		// if err != nil {
		// 	log.Printf("Error marshalling to JSON: %v", err)
		// } else {
		// 	// Выводим информацию об устройстве
		// 	log.Printf("[UDP] Device info:\n%s", string(jsonData))
		// }
	}

	evbuf.Push(events.Event{
		DeviceID: dev.UID,
		Topic:    "system/discovery",
		Payload:  []byte(fmt.Sprintf("Device %s discovered at %s", dev.UID, addr.IP.String())),
		Time:     time.Now(),
	})
}

func handleEvent(data []byte, addr *net.UDPAddr, reg *registry.Store, evbuf events.Buffer) {
	r := bytes.NewReader(data)
	var msg BinaryEventPacket

	if err := binary.Read(r, binary.LittleEndian, &msg); err != nil {
		log.Printf("[UDP] Ошибка парсинга event: %v", err)
		return
	}

	deviceID := "unknown"
	remoteAddr := fmt.Sprintf("%s:%d", addr.IP.String(), addr.Port)
	for _, dev := range reg.List() {
		if dev.AdapterDS == remoteAddr {
			deviceID = dev.UID
			break
		}
	}

	// Генерация картинки и кодирование в Base64
	var base64Image string
	imgBytes, err := generateZoneImage(&msg.Zones)
	if err != nil {
		log.Printf("[UDP] Ошибка генерации картинки: %v", err)
	} else {
		base64Image = base64.StdEncoding.EncodeToString(imgBytes)
	}

	// Формирование Payload для системы (JSON с данными + картинка)
	payloadMap := map[string]interface{}{
		"data":  msg,
		"image": base64Image,
	}

	jsonBytes, err := json.Marshal(payloadMap)
	if err != nil {
		log.Printf("[UDP] Ошибка формирования JSON: %v", err)
		return
	}

	// Отправка в шину событий
	evbuf.Push(events.Event{
		DeviceID: deviceID,
		Topic:    "detector/event",
		Payload:  jsonBytes,
		Time:     time.Now(),
	})

	// Запись логов в CSV
	logFileName := fmt.Sprintf("detector_logs_%s.csv", time.Now().Format("02.01.2006"))
	logPath := filepath.Join(".", logFileName)
	if err := writeToCSV(logPath, deviceID, addr.IP.String(), &msg); err != nil {
		log.Printf("[CSV] Ошибка записи лога: %v", err)
	} else {
		log.Printf("[CSV] Запись лога в %s", logPath)
	}

}
