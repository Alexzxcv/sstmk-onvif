# UDP Adapter

Адаптер для обнаружения и мониторинга детекторов металла через бинарный UDP протокол.

## Архитектура

### adapter.go - Transport Layer
Управление UDP соединением и маршрутизация команд.

**Функции:**
- Создание UDP сервера на порту 50000
- Broadcast discovery каждые 10 секунд
- Прием и маршрутизация входящих пакетов
- Фильтрация собственных broadcast-запросов

### protocol.go - Protocol Layer
Определение структур бинарного протокола.

**Содержит:**
- Константы команд (DISCOVERY, EVENT_NOTIFICATION, ACK)
- `BinaryDiscoveryPacket` - информация об устройстве
- `BinaryEventPacket` - события детектора
- `DetectorStatus` - статус детектора (проходы, скорость, металл)
- `DetectorZones` - сетка зон 6×2 с данными тревог

### handler.go - Business Logic
Обработка входящих сообщений и координация компонентов.

**Функции:**
- `handleRegistration()` - регистрация устройств в реестре
- `handleEvent()` - обработка событий детектора
  - Парсинг бинарных данных
  - Поиск устройства по IP:Port
  - Генерация изображения зон
  - Отправка в event bus
  - Запись в CSV лог

### visualizer.go - Visualization
Генерация визуализации зон детектора.

**Функции:**
- `generateZoneImage()` - создает PNG изображение 100×300px
  - Красный цвет = активная зона
  - Серый цвет = неактивная зона
  - Черные границы между ячейками
- Кодирование в base64 для передачи через JSON

### logger.go - CSV Logging
Запись событий детектора в CSV файл.

**Функции:**
- `writeToCSV()` - добавление записи в лог
  - Автоматическое создание файла с заголовками
  - Разделитель: точка с запятой (;)
  - Формат времени: RFC3339

## Использование

```go
import "sstmk-onvif/internal/adapters/udp"

err := udp.Start(ctx, registry, eventBuffer)
```

## Протокол

**Порт:** 50000 UDP  
**Формат:** Little Endian binary  
**Команды:**
- `0x00` - Discovery Request/Response
- `0x05` - Event Notification
- `0xFF` - ACK