package adapters

import "context"

type Sink interface {
	OnRaw(deviceID string, payload []byte) // куда сыпать сырые данные (дальше нормализуем → events.Buffer)
}

type Adapter interface {
	Start(ctx context.Context) error
}

type Factory func(deviceID, datasource string, sink Sink) (Adapter, error)
