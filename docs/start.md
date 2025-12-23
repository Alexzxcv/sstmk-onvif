WS-Discovery {
    port = 3702
    ip = 239.255.255.250
}

HTTP(SOAP) port = 8888

конфиг go (windows)
    $env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="mipsle"; $env:GOMIPS="softfloat"
сборка
    go clean; go build -trimpath -ldflags="-s -w" -o sstmk-onvif .\cmd\sstmk-onvif\
сделать исполняемым
    chmod +x /tmp/sstmk-onvif
запуск
    /tmp/sstmk-onvif -addr :10000 -host http://192.168.16.254:10000 &
остановка
    killall sstmk-onvif