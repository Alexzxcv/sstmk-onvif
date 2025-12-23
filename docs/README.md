sstmk-onvif/
├─ cmd/
│  └─ sstmk-onvif/
│     └─ main.go                     # точка входа (парсер флагов, запуск discovery + http-серверов)
├─ internal/
│  ├─ config/
│  │  ├─ config.go                   # парсинг YAML/ENV/UCI, валидация
│  │  └─ defaults.go                 # значения по умолчанию
│  ├─ registry/
│  │  ├─ registry.go                 # модель DevMeta, хранение, индексы, поиск
│  │  └─ load_from_config.go         # сбор реестра из конфигурации
│  ├─ discovery/
│  │  ├─ wsdiscovery.go              # UDP 3702, joinGroup, чтение Probe
│  │  └─ responder.go                # сбор ProbeMatches, чанкование по MTU
│  ├─ httpdev/
│  │  ├─ server.go                   # запуск http.Server per device (порт)
│  │  ├─ handlers.go                 # GetCapabilities/GetServices/GetScopes/GetDeviceInformation
│  │  └─ soap_templates.go           # SOAP-обёртки/шаблоны
│  ├─ events/
│  │  ├─ pullpoint.go                # (позже) CreatePullPointSubscriptionPullMessages
│  │  └─ buffer.go                   # буфер событий/топики
│  ├─ adapters/
│  │  ├─ tcp/
│  │  │  └─ adapter.go               # драйвер для рамки/ручного по TCP
│  │  ├─ udp/
│  │  │  └─ adapter.go               # (если нужно)
│  │  └─ mock/
│  │     └─ adapter.go               # «заглушка» для разработки
│  ├─ logging/
│  │  └─ logger.go                   # единая настройка логов
│  ├─ util/
│  |  ├─ net.go                      # guessLocalIP, helpers
│  |  └─ uuid.go                     # urn:uuid, xmlEscape
│  └─ web/
│     ├─ handlers.go                 # обработка api
│     └─ server.go                   # веб сервер для admin panel
├─ pkg/
│  └─ onvif/
│     └─ constants.go                # неймспейсы, типы, константы
├─ configs/
│  ├─ sstmk-onvif.yml                # основной YAML-конфиг
│  └─ uci/
│     └─ sstmk-onvif                 # /etc/config/sstmk-onvif (альтернативно UCI)
├─ openwrt/
│  ├─ init.d/
│  │  └─ sstmk-onvif                 # /etc/init.d/sstmk-onvif (procd-скрипт)
│  ├─ firewall.rules                 # пример iptables-правил
│  └─ Makefile                       # OpenWrt package (опционально)
├─ scripts/
│  ├─ build-mipsle.sh                # сборка для OpenWrt (mipsle softfloat)
│  ├─ run-local.sh                   # локальный запуск (darwin/linux/amd64)
│  └─ push-to-router.sh              # копирование на роутер + запуск
├─ docs/
│  ├─ README.md
│  ├─ CONFIG.md
│  └─ PROTOCOLS.md                   # описание низкоуровневых протоколов/маппинга
├─ go.mod
└─ go.sum