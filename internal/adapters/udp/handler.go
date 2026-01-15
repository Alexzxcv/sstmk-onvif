package udp

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"sstmk-onvif/internal/events"
	"sstmk-onvif/internal/registry"
)

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

	r := bytes.NewReader(data)

	var msg BinaryEventPacket

	binary.Read(r, binary.LittleEndian, &msg)

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
		fileName := fmt.Sprintf("./event_%d.png", msg.TS)
		if err := os.WriteFile(fileName, imgBytes, 0644); err != nil {
			log.Printf("[UDP] Ошибка сохранения картинки %s: %v", fileName, err)
		}
	}
	// ----только для лога
	msgEv, _ := json.MarshalIndent(msg, "", "  ")
	fmt.Println(string(msgEv))
	// -------------------

	// 4. Формирование Payload для системы (JSON с данными + картинка)
	// Создаем структуру, объединяющую данные события и картинку
	payloadMap := map[string]interface{}{
		"data":  msg,         // Исходные данные пакета
		"image": base64Image, // Картинка в base64 (пустая строка, если ошибка)
	}

	jsonBytes, err := json.Marshal(payloadMap)

	if err != nil {
		log.Printf("[UDP] Ошибка формирования JSON: %v", err)
	} else {
		// Отправка в шину событий
		ev := events.Event{
			DeviceID: deviceID,
			Topic:    "detector/event",
			Payload:  jsonBytes,
			Time:     time.Now(),
		}
		evbuf.Push(ev)
	}

	// 5. Запись логов на флешку (CSV) - без изменений
	const logPath = "./detector_logs.csv"
	if err := writeToCSV(logPath, deviceID, addr.IP.String(), &msg); err != nil {
		log.Printf("[CSV] Ошибка записи лога: %v", err)
	}
}
