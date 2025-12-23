package httpdev

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"sstmk-onvif/internal/config"
	"sstmk-onvif/internal/registry"
)

func StartAll(ctx context.Context, cfg *config.Config, reg *registry.Store) error {
	for _, m := range reg.List() {
		m := m // capture
		mux := http.NewServeMux()

		devicePath := cfg.DevicePath
		eventsPath := cfg.EventsPath

		mux.HandleFunc(devicePath, func(w http.ResponseWriter, r *http.Request) {
			deviceServiceHandlerFor(w, r, m, cfg.PublicIP, devicePath, eventsPath)
		})
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == devicePath {
				deviceServiceHandlerFor(w, r, m, cfg.PublicIP, devicePath, eventsPath)
				return
			}
			http.NotFound(w, r)
		})

		srv := &http.Server{
			Addr:              fmt.Sprintf(":%d", m.Port),
			Handler:           logMiddleware(mux),
			ReadHeaderTimeout: 5 * time.Second,
		}

		go func(id string) {
			log.Printf("httpdev: %s listening on :%d", id, m.Port)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("httpdev: server %s error: %v", id, err)
			}
		}(m.ID)

		// graceful shutdown
		go func() {
			<-ctx.Done()
			shCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = srv.Shutdown(shCtx)
		}()
	}
	return nil
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		buf := new(bytes.Buffer)
		tee := io.TeeReader(r.Body, buf)
		body, _ := io.ReadAll(tee)
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))

		log.Printf(">> %s %s on %s from %s | len=%d\n%s",
			r.Method, r.URL.Path, r.Host, r.RemoteAddr, len(body), truncate(body, 800))
		next.ServeHTTP(w, r)
		log.Printf("<< %s %s handled in %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func truncate(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "\n... [truncated] ..."
}

func deviceServiceHandlerFor(w http.ResponseWriter, r *http.Request, m registry.Device, pubIP, devicePath, eventsPath string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()
	req := string(body)

	host := pubIP
	if host == "" {
		if h, _, err := net.SplitHostPort(r.Host); err == nil && h != "" {
			host = h
		}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	base := fmt.Sprintf("http://%s:%d", host, m.Port)
	devX := base + devicePath
	evX := base + eventsPath

	var resp string
	switch {
	case strings.Contains(req, "<GetCapabilities"):
		resp = soapResponseGetCapabilities(devX, evX)
	case strings.Contains(req, "<GetServices"):
		resp = soapResponseGetServices(devX, evX)
	case strings.Contains(req, "<GetScopes"):
		resp = soapResponseGetScopesFor(m)
	case strings.Contains(req, "<GetDeviceInformation"):
		resp = soapResponseGetDeviceInformationFor(m)
	default:
		resp = soapFaultUnsupported()
	}

	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	_, _ = w.Write([]byte(resp))
}

/* ---------- SOAP helpers ---------- */

func soapEnvelope(body string) string {
	return fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?>
<env:Envelope xmlns:env="http://www.w3.org/2003/05/soap-envelope">
  <env:Header/>
  <env:Body>
%s
  </env:Body>
</env:Envelope>`, body)
}

func soapResponseGetCapabilities(deviceXAddr, eventsXAddr string) string {
	b := fmt.Sprintf(`
<tds:GetCapabilitiesResponse xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
  <tds:Capabilities xmlns:tt="http://www.onvif.org/ver10/schema">
    <tt:Device><tt:XAddr>%s</tt:XAddr></tt:Device>
    <tt:Events>
      <tt:XAddr>%s</tt:XAddr>
      <tt:WSSubscriptionPolicySupport>true</tt:WSSubscriptionPolicySupport>
      <tt:WSPullPointSupport>true</tt:WSPullPointSupport>
    </tt:Events>
  </tds:Capabilities>
</tds:GetCapabilitiesResponse>`, deviceXAddr, eventsXAddr)
	return soapEnvelope(b)
}

func soapResponseGetServices(deviceXAddr, eventsXAddr string) string {
	b := fmt.Sprintf(`
<tds:GetServicesResponse xmlns:tds="http://www.onvif.org/ver10/device/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
  <tds:Service>
    <tds:Namespace>http://www.onvif.org/ver10/device/wsdl</tds:Namespace>
    <tds:XAddr>%s</tds:XAddr>
    <tds:Version><tt:Major>2</tt:Major><tt:Minor>60</tt:Minor></tds:Version>
  </tds:Service>
  <tds:Service>
    <tds:Namespace>http://www.onvif.org/ver10/events/wsdl</tds:Namespace>
    <tds:XAddr>%s</tds:XAddr>
    <tds:Version><tt:Major>2</tt:Major><tt:Minor>60</tt:Minor></tds:Version>
  </tds:Service>
</tds:GetServicesResponse>`, deviceXAddr, eventsXAddr)
	return soapEnvelope(b)
}

func soapResponseGetScopesFor(m registry.Device) string {
	b := fmt.Sprintf(`
<tds:GetScopesResponse xmlns:tds="http://www.onvif.org/ver10/device/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
  <tds:Scopes><tt:ScopeDef>Fixed</tt:ScopeDef><tt:ScopeItem>onvif://www.onvif.org/name/%s</tt:ScopeItem></tds:Scopes>
  <tds:Scopes><tt:ScopeDef>Fixed</tt:ScopeDef><tt:ScopeItem>onvif://www.onvif.org/type/%s</tt:ScopeItem></tds:Scopes>
  <tds:Scopes><tt:ScopeDef>Fixed</tt:ScopeDef><tt:ScopeItem>onvif://www.onvif.org/location/%s</tt:ScopeItem></tds:Scopes>
</tds:GetScopesResponse>`, m.Name, m.TypeScope, m.Location)
	return soapEnvelope(b)
}

func soapResponseGetDeviceInformationFor(m registry.Device) string {
	b := fmt.Sprintf(`
<tds:GetDeviceInformationResponse xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
  <tds:Manufacturer>%s</tds:Manufacturer>
  <tds:Model>%s</tds:Model>
  <tds:FirmwareVersion>%s</tds:FirmwareVersion>
  <tds:SerialNumber>%s</tds:SerialNumber>
  <tds:HardwareId>%s</tds:HardwareId>
</tds:GetDeviceInformationResponse>`, m.Vendor, m.Model, m.Firmware, m.Serial, m.Hardware)
	return soapEnvelope(b)
}

func soapFaultUnsupported() string {
	return soapEnvelope(`
<env:Fault xmlns:env="http://www.w3.org/2003/05/soap-envelope">
  <env:Code><env:Value>env:Sender</env:Value></env:Code>
  <env:Reason><env:Text xml:lang="en">Unsupported or unknown request</env:Text></env:Reason>
</env:Fault>`)
}
