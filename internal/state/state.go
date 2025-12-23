package state

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sstmk-onvif/internal/config"
)

type State struct {
	Devices []config.Device `json:"devices"`
}

func LoadOrInit(path string, cfgDevices []config.Device) (*State, error) {

	_, err := os.Stat(path)

	if os.IsNotExist(err) {

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("ошибка создания пути: %w", err)
		}

		initData := State{
			Devices: cfgDevices,
		}

		data, err := json.MarshalIndent(initData, "", "  ")

		if err != nil {
			return nil, fmt.Errorf("ошибка формирования json: %w", err)
		}

		err = os.WriteFile(path, data, 0644)

		if err != nil {
			return nil, fmt.Errorf("ошибка записи данных: %w", err)
		}
		log.Printf("Инициализация данных успешно завершена\n")

		return &initData, nil

	} else if err != nil {
		// ТУТ НУЖНО ПРОБРОСИТЬ ИСКЛЮЧЕНИЕ НАВЕРХ
		return nil, fmt.Errorf("фатальная ошибка при проверке файла %s: %w", path, err)
		//----------------------
	} else {
		// --- Путь 3: Файл существует. Загружаем его. ---
		log.Printf("файл '%s' существует, загрузка...\n", path)

		// ЗДЕСЬ НУЖНА ЛОГИКА ЧТЕНИЯ ФАЙЛА
		fileData, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("ошибка чтения существующего файла состояния: %w", readErr)
		}

		var stateFromFile State
		if unmarshalErr := json.Unmarshal(fileData, &stateFromFile); unmarshalErr != nil {
			return nil, fmt.Errorf("ошибка парсинга JSON существующего файла состояния: %w", unmarshalErr)
		}

		// Успешный возврат загруженного состояния
		return &stateFromFile, nil
	}
}

func SaveDevices(path string, devices []config.Device) error {
	st := State{
		Devices: devices,
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ошибка создания каталога для state.json: %w", err)
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("ошибка кодирования JSON state: %w", err)
	}

	// Пишем во временный файл и атомарно переименовываем
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("ошибка записи временного файла state: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("ошибка замены state-файла: %w", err)
	}

	return nil
}
