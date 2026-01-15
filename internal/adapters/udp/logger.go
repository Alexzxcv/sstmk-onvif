package udp

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"
)

func writeToCSV(filepath string, deviceID, ip string, msg *BinaryEventPacket) error {
	// 1. Проверяем, существует ли файл, чтобы понять, нужно ли писать заголовок
	fileExists := false
	if _, err := os.Stat(filepath); err == nil {
		fileExists = true
	}

	// 2. Открываем файл:
	// os.O_APPEND - дописывать в конец
	// os.O_CREATE - создать, если нет
	// os.O_WRONLY - только для записи
	f, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл логов: %w", err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	writer.Comma = ';'
	defer writer.Flush()

	// 3. Если файл новый, пишем строку заголовков
	if !fileExists {
		header := []string{
			"Time", "DeviceID", "IP",
			"State", "In", "Out", "Inside", "Speed", "Level",
			"MetalAlarms",
		}

		if err := writer.Write(header); err != nil {
			return fmt.Errorf("ошибка записи заголовка CSV: %w", err)
		}
	}

	// 4. Формируем строку данных
	record := []string{
		time.Now().Format(time.RFC3339),
		deviceID,
		ip,
		fmt.Sprintf("%d", msg.Status.State),
		fmt.Sprintf("%d", msg.Status.In),
		fmt.Sprintf("%d", msg.Status.Out),
		fmt.Sprintf("%d", msg.Status.Inside),
		fmt.Sprintf("%.2f", msg.Status.Speed),
		fmt.Sprintf("%d", msg.Status.Level),
		fmt.Sprintf("%d", msg.Status.Metal.Alarms),
	}

	// 5. Записываем строку
	if err := writer.Write(record); err != nil {
		return fmt.Errorf("ошибка записи данных CSV: %w", err)
	}

	return nil
}
