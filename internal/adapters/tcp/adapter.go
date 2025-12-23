package tcp

import (
	"bufio"
	"context"
	"errors"
	"net"
	"time"

	"sstmk-onvif/internal/adapters"
)

type tcpAdapter struct {
	deviceID string
	ds       string
	sink     adapters.Sink
}

func New(deviceID, ds string, sink adapters.Sink) (adapters.Adapter, error) {
	return &tcpAdapter{deviceID: deviceID, ds: ds, sink: sink}, nil
}

func (a *tcpAdapter) Start(ctx context.Context) error {
	var d net.Dialer
	backoff := time.Second
	for {
		// context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := d.DialContext(ctx, "tcp", a.ds)
		if err != nil {
			// backoff and retry
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			if backoff < 10*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second

		// read loop
		_ = conn.SetReadDeadline(time.Time{})
		reader := bufio.NewReader(conn)
		buf := make([]byte, 4096)
		for {
			// cooperative cancellation
			if err := ctx.Err(); err != nil {
				_ = conn.Close()
				return err
			}
			n, err := reader.Read(buf)
			if n > 0 {
				// copy payload before next read
				b := make([]byte, n)
				copy(b, buf[:n])
				a.sink.OnRaw(a.deviceID, b)
			}
			if err != nil {
				_ = conn.Close()
				break // reconnect
			}
		}
	}
}
