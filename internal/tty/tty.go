// internal/tty/tty.go
package tty

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"sstmk-onvif/internal/events"
)

// Пока минимальный конфиг — просто путь до устройства.
// Потом можно вынести в общий config.Config.
type Config struct {
	Device string
}

// Start блокирующе читает строки из tty и пушит их в evbuf.
// Формат строки: EVT,<input_id>,<state>\n
func Start(ctx context.Context, cfg Config, evbuf events.Buffer) error {
	if cfg.Device == "" {
		log.Printf("tty: disabled (no device)")
		<-ctx.Done()
		return ctx.Err()
	}

	f, err := os.Open(cfg.Device)
	if err != nil {
		return fmt.Errorf("tty: cannot open %s: %w", cfg.Device, err)
	}
	defer f.Close()

	log.Printf("[tty]: listening on %s", cfg.Device)

	// Чтобы ctx отменял блокирующее чтение — закрываем fd,
	// когда контекст завершится.
	go func() {
		<-ctx.Done()
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)

	for {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				// Если нас закрыли через ctx — выходим тихо.
				if ctx.Err() != nil {
					log.Printf("tty: stopped (ctx): %v", ctx.Err())
					return ctx.Err()
				}
				return fmt.Errorf("tty: read error: %w", err)
			}
			// EOF без ошибки — немного подождём и продолжим
			time.Sleep(100 * time.Millisecond)
			continue
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Ожидаем: EVT,<id>,<state>
		parts := strings.Split(line, ",")
		if len(parts) != 3 || parts[0] != "EVT" {
			log.Printf("tty: ignore line %q", line)
			continue
		}
		log.Printf("Data: %s", parts)
		inputID, err1 := strconv.Atoi(parts[1])
		state, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil {
			log.Printf("tty: bad ints in %q", line)
			continue
		}

		// Минимальный payload — JSON, который дальше можно парсить в UI.
		payload := []byte(fmt.Sprintf(`{"input":%d,"state":%d}`, inputID, state))
		log.Printf("%s", payload)
		ev := events.Event{
			DeviceID: fmt.Sprintf("tty-input-%d", inputID), // можешь поменять на "gate-001" и т.п.
			Topic:    "input",
			Payload:  payload,
			Time:     time.Now(),
		}
		evbuf.Push(ev)
	}
}

/*
test:
socat -d -d \
  pty,raw,echo=0,link=/tmp/ttyV0 \
  pty,raw,echo=0,link=/tmp/ttyV1 &


  echo -e "EVT,3,1" > /tmp/ttyV1
*/
