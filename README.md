# SSTMK-ONVIF

Система мониторинга детекторов металла с поддержкой ONVIF протокола.

## Описание

SSTMK-ONVIF - это сервер для интеграции детекторов металла в системы видеонаблюдения через протокол ONVIF. Поддерживает обнаружение устройств, получение событий и визуализацию данных детекторов.

## Архитектура

```
sstmk-onvif/
├── cmd/                    # Точки входа приложений
│   ├── sstmk-onvif/       # Основной сервер
│   └── frame-emulator/    # Эмулятор для тестирования
├── internal/              # Внутренние модули
│   ├── adapters/          # Адаптеры протоколов
│   ├── bootstrap/         # Инициализация приложения
│   ├── config/            # Конфигурация
│   ├── discovery/         # WS-Discovery
│   ├── events/            # Шина событий
│   ├── httpdev/           # HTTP API для устройств
│   ├── hub/               # WebSocket hub
│   ├── registry/          # Реестр устройств
│   ├── state/             # Управление состоянием
│   ├── tty/               # TTY адаптер
│   └── web/               # Web интерфейс
├── webui/                 # Веб-интерфейс
├── docs/                  # Документация
└── configs/               # Конфигурационные файлы
```

## Модули

### Adapters - Адаптеры протоколов
- **[UDP Adapter](internal/adapters/udp/docs.md)** - Бинарный UDP протокол для детекторов
- **[TCP Adapter](internal/adapters/tcp/docs.md)** - TCP протокол (в разработке)

### Core - Основные компоненты
- **[Bootstrap](internal/bootstrap/docs.md)** - Инициализация и запуск всех сервисов
- **[Config](internal/config/docs.md)** - Управление конфигурацией
- **[Events](internal/events/docs.md)** - Шина событий для межмодульной коммуникации
- **[Registry](internal/registry/docs.md)** - Реестр обнаруженных устройств

### Network - Сетевые сервисы
- **[Discovery](internal/discovery/docs.md)** - WS-Discovery для ONVIF
- **[HTTP Dev](internal/httpdev/docs.md)** - HTTP API для управления устройствами
- **[Web Server](internal/web/docs.md)** - Веб-интерфейс и WebSocket

### Infrastructure
- **[Hub](internal/hub/docs.md)** - WebSocket hub для real-time обновлений
- **[State](internal/state/docs.md)** - Управление состоянием системы
- **[TTY](internal/tty/docs.md)** - Адаптер для последовательного порта

## Быстрый старт

### Установка

```bash
git clone <repository>
cd sstmk-onvif
go mod download
```

### Запуск

```bash
# Основной сервер
go run cmd/sstmk-onvif/main.go

# Эмулятор (для тестирования)
go run cmd/frame-emulator/main.go
```

### Конфигурация

Конфигурационный файл: `configs/sstmk-onvif.yml`

```yaml
server:
  port: 8080
udp:
  port: 50000
  discovery_interval: 10s
```

## Протоколы

### UDP Binary Protocol
- Порт: 50000
- Формат: Little Endian binary
- Команды: Discovery (0x00), Event Notification (0x05)
- Подробнее: [Binary API](docs/binary_api.md)

### ONVIF
- WS-Discovery для обнаружения устройств
- Event Service для уведомлений
- Подробнее: [ONVIF Integration](docs/onvif.md)

## API

### HTTP API
```
GET  /api/devices          # Список устройств
GET  /api/devices/:id      # Информация об устройстве
POST /api/devices/:id/cmd  # Отправка команды
```

### WebSocket
```
ws://localhost:8080/ws     # Real-time события
```

## Разработка

### Структура кода
- Следуем Clean Architecture
- Разделение на слои: Transport → Handler → Domain
- Dependency Injection через параметры функций

### Тестирование
```bash
go test ./...
```

### Сборка
```bash
go build -o sstmk-onvif cmd/sstmk-onvif/main.go
```

## Документация

- [Начало работы](docs/start.md)
- [Binary API](docs/binary_api.md)
- [TTY Protocol](docs/tty.md)
- [Архитектура UDP Adapter](internal/adapters/udp/docs.md)

## Лицензия

[Указать лицензию]

## Контакты

[Указать контакты]
