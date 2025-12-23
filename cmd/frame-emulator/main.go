// cmd/frame-emulator/main.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Command struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type PingResp struct {
	Pong     bool      `json:"pong"`
	TS       int64     `json:"ts"`
	Commands []Command `json:"commands"`
}

func main() {
	var (
		baseURL   = flag.String("url", "http://127.0.0.1:8080/api/v1", "Base API URL (router server)")
		id        = flag.String("id", "gate-emu-001", "Device ID")
		fw        = flag.String("fw", "1.0.0-emu", "Firmware version string")
		pingEvery = flag.Duration("ping", 3*time.Second, "Ping interval")
		statEvery = flag.Duration("status", 2*time.Second, "Status push interval")
		jitterPct = flag.Float64("jitter", 0.2, "Jitter percent for intervals (0..1)")
		timeout   = flag.Duration("lp-timeout", 25*time.Second, "Long-poll timeout")
		auth      = flag.String("auth", "", "Authorization token (optional, Bearer <token>)")
		verbose   = flag.Bool("v", true, "Verbose logs")
	)
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	cl := &http.Client{Timeout: *timeout + 5*time.Second}

	// graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if *verbose {
		log.Printf("frame-emulator started id=%s base=%s fw=%s", *id, *baseURL, *fw)
	}

	// горутина: long-poll ping-pong
	go func() {
		ackBuf := make([]string, 0, 64)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			reqBody := map[string]any{
				"fw":  *fw,
				"ack": ackBuf,
				// можно слать легкие метрики прямо тут, если хочешь
				"metrics": map[string]any{
					"temp":     36.5 + 3*rand.Float64(),
					"voltage":  4.8 + 0.4*rand.Float64(),
					"restarts": rand.Intn(3),
				},
			}
			ackBuf = ackBuf[:0]

			var buf bytes.Buffer
			_ = json.NewEncoder(&buf).Encode(reqBody)

			url := fmt.Sprintf("%s/devices/%s/ping", strings.TrimRight(*baseURL, "/"), *id)
			resp, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
			if err != nil {
				log.Printf("ping new req: %v", err)
				sleepWithJitter(*pingEvery, *jitterPct)
				continue
			}
			resp.Header.Set("Content-Type", "application/json")
			if *auth != "" {
				resp.Header.Set("Authorization", "Bearer "+*auth)
			}

			res, err := cl.Do(resp)
			if err != nil {
				if *verbose {
					log.Printf("ping error: %v", err)
				}
				sleepWithJitter(backoff(*pingEvery), *jitterPct)
				continue
			}
			body, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			if res.StatusCode/100 != 2 {
				if *verbose {
					log.Printf("ping HTTP %d: %s", res.StatusCode, string(body))
				}
				sleepWithJitter(backoff(*pingEvery), *jitterPct)
				continue
			}

			var pr PingResp
			if err := json.Unmarshal(body, &pr); err != nil {
				if *verbose {
					log.Printf("ping decode: %v body=%s", err, string(body))
				}
				sleepWithJitter(*pingEvery, *jitterPct)
				continue
			}
			if len(pr.Commands) > 0 && *verbose {
				log.Printf("commands: %d", len(pr.Commands))
			}

			// “исполняем” команды
			for _, c := range pr.Commands {
				if *verbose {
					log.Printf("cmd: id=%s type=%s payload=%s", c.ID, c.Type, string(c.Payload))
				}
				switch strings.ToLower(c.Type) {
				case "reboot":
					go simulateReboot(*id, *verbose)
				case "setparam":
					// пример payload: {"name":"threshold","value":123}
					if *verbose {
						log.Printf("setparam applied (fake): %s", string(c.Payload))
					}
				case "alarmtest":
					if *verbose {
						log.Printf("alarmtest: triggering fake alarm for 2s")
					}
				default:
					if *verbose {
						log.Printf("unknown command: %s", c.Type)
					}
				}
				// добавим ID в ack на следующий ping
				ackBuf = append(ackBuf, c.ID)
			}

			// Следующий ping после краткой паузы (long-poll уже ждал до timeout)
			sleepWithJitter(*pingEvery, *jitterPct)
		}
	}()

	// горутина: периодический POST статуса (формат близкий к твоему C JSON)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			status := generateStatus()
			body := map[string]any{
				"status": status,
			}
			var buf bytes.Buffer
			_ = json.NewEncoder(&buf).Encode(body)

			url := fmt.Sprintf("%s/devices/%s/status", strings.TrimRight(*baseURL, "/"), *id)
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
			req.Header.Set("Content-Type", "application/json")
			if *auth != "" {
				req.Header.Set("Authorization", "Bearer "+*auth)
			}

			res, err := cl.Do(req)
			if err != nil {
				log.Printf("status post error: %v", err)
			} else {
				_, _ = io.Copy(io.Discard, res.Body)
				_ = res.Body.Close()
			}

			sleepWithJitter(*statEvery, *jitterPct)
		}
	}()

	<-ctx.Done()
	log.Printf("frame-emulator stopped")
}

func sleepWithJitter(base time.Duration, pct float64) {
	if pct <= 0 {
		time.Sleep(base)
		return
	}
	delta := base.Seconds() * pct
	j := (rand.Float64()*2 - 1) * delta // [-pct..+pct]
	time.Sleep(time.Duration((base.Seconds() + j) * float64(time.Second)))
}

func backoff(base time.Duration) time.Duration {
	// простой backoff x2 до 10s
	b := base * 2
	if b > 10*time.Second {
		return 10 * time.Second
	}
	return b
}

func simulateReboot(id string, verbose bool) {
	if verbose {
		log.Printf("[%s] rebooting (fake) ...", id)
	}
	time.Sleep(2 * time.Second)
	if verbose {
		log.Printf("[%s] reboot done", id)
	}
}

// ----- генерация статуса, совместимого по структуре с твоим JSON -----

func generateStatus() map[string]any {
	const H = 6 // горизонтальные зоны (пример)
	const V = 8 // вертикальные зоны (пример)

	boolGrid := func() [][]bool {
		out := make([][]bool, H)
		for i := 0; i < H; i++ {
			row := make([]bool, V)
			for j := 0; j < V; j++ {
				row[j] = rand.Intn(5) == 0 // 20% true
			}
			out[i] = row
		}
		return out
	}
	u8Grid := func(max int) [][]uint8 {
		out := make([][]uint8, H)
		for i := 0; i < H; i++ {
			row := make([]uint8, V)
			for j := 0; j < V; j++ {
				row[j] = uint8(rand.Intn(max))
			}
			out[i] = row
		}
		return out
	}
	u32Grid := func(max int) [][]uint32 {
		out := make([][]uint32, H)
		for i := 0; i < H; i++ {
			row := make([]uint32, V)
			for j := 0; j < V; j++ {
				row[j] = uint32(rand.Intn(max))
			}
			out[i] = row
		}
		return out
	}

	section := func() map[string]any {
		return map[string]any{
			"alarms":    rand.Uint32() & 0xF,
			"alarm_in":  rand.Uint32() & 0x3,
			"alarm_out": rand.Uint32() & 0x3,
			"level":     rand.Uint32() & 0xFF,
		}
	}

	zones := func() map[string]any {
		return map[string]any{
			"alarm": boolGrid(),
			"level": u8Grid(200),
			"cnt":   u32Grid(1000),
		}
	}

	return map[string]any{
		"status": map[string]any{
			"state":         "RUN",
			"in":            rand.Uint32() % 20,
			"out":           rand.Uint32() % 20,
			"inside":        rand.Uint32() % 20,
			"speed":         0.5 + rand.Float64()*2.5,
			"calib_timeout": rand.Uint32() % 120,
			"general":       section(),
			"main":          section(),
			"ext":           section(),
		},
		"zones": map[string]any{
			"general": zones(),
			"main":    zones(),
			"ext":     zones(),
		},
	}
}
