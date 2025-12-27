package discovery

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"golang.org/x/net/ipv4"

	"sstmk-onvif/internal/config"
	"sstmk-onvif/internal/registry"
)

func Start(ctx context.Context, cfg *config.Config, reg *registry.Store) error {
	go runWSDiscovery(ctx, cfg, reg)
	return nil
}

func runWSDiscovery(ctx context.Context, cfg *config.Config, reg *registry.Store) {
	pc, err := net.ListenPacket("udp4", "0.0.0.0:3702")
	if err != nil {
		log.Printf("ws-discovery: listen error: %v", err)
		return
	}
	defer pc.Close()
	log.Printf("ws-discovery: listening on UDP :3702")

	p := ipv4.NewPacketConn(pc)
	_ = p.SetControlMessage(ipv4.FlagDst, true)
	_ = p.SetMulticastLoopback(true)

	if ifi, err := net.InterfaceByName(cfg.LANIfName); err != nil {
		log.Printf("ws-discovery: cannot find iface %s: %v", cfg.LANIfName, err)
	} else {
		mcast := net.IPv4(239, 255, 255, 250)
		if err := p.JoinGroup(ifi, &net.UDPAddr{IP: mcast}); err != nil {
			log.Printf("ws-discovery: JoinGroup on %s failed: %v", cfg.LANIfName, err)
		} else {
			log.Printf("ws-discovery: joined %s on %s", mcast.String(), cfg.LANIfName)
		}
		_ = p.SetMulticastInterface(ifi)
		_ = p.SetMulticastTTL(1)
	}

	buf := make([]byte, 8192)
	const maxUDP = 1300

	for {
		// cancellation?
		if err := ctx.Err(); err != nil {
			return
		}

		_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, cm, raddr, err := p.ReadFrom(buf)
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			continue
		}
		if err != nil {
			log.Printf("ws-discovery: read error: %v", err)
			continue
		}

		_ = cm // destination info не используем здесь

		req := string(buf[:n])

		log.Printf("ws-discovery: received %d bytes from %s", n, raddr)

		log.Printf("DATA=%s", req)
		if !looksLikeProbe(req) {
			log.Printf("Не прошла проверку")
			continue
		}

		relates := extractTag(req, "MessageID")
		if relates == "" {
			relates = uuidURN()
		}

		// IP в XAddr: public_ip (если задан) или автоопределение
		localIP := cfg.PublicIP
		if localIP == "" {
			localIP = guessLocalIP(raddr)
		}

		// соберём матчи
		var chunkBody strings.Builder
		sendChunk := func() {
			if chunkBody.Len() == 0 {
				log.Printf("Расходимся")
				return
			}
			resp := envelope(relates, chunkBody.String())
			_, _ = p.WriteTo([]byte(resp), nil, raddr)
			chunkBody.Reset()
			time.Sleep(10 * time.Millisecond)
		}
		log.Println("List=", reg.List())
		for _, m := range reg.List() {
			// ⬇️ НЕ показываем устройство, если оно выключено или оффлайн
			if !m.Enabled || !m.Online {
				continue
			}

			x := fmt.Sprintf("http://%s:%d%s", localIP, m.Port, cfg.DevicePath)
			scopes := fmt.Sprintf(
				"onvif://www.onvif.org/name/%s onvif://www.onvif.org/type/%s",
				m.Name, m.Model,
			)

			one := matchXML(probeMatch{
				Endpoint: uuidURN(), // можно сделать стабильным, если захочешь (см. ниже)
				XAddr:    x,
				Scopes:   scopes,
			})

			if chunkBody.Len() > 0 && (chunkBody.Len()+len(one) > maxUDP) {
				sendChunk()
			}
			chunkBody.WriteString(one)
		}
		sendChunk()
	}
}

/* ---------- helpers (локальные, чтобы не плодить зависимостей) ---------- */

type probeMatch struct {
	Endpoint, XAddr, Scopes string
}

func matchXML(m probeMatch) string {
	return fmt.Sprintf(
		`<d:ProbeMatch><a:EndpointReference><a:Address>%s</a:Address></a:EndpointReference><d:Types>dn:NetworkVideoTransmitter</d:Types><d:Scopes>%s</d:Scopes><d:XAddrs>%s</d:XAddrs><d:MetadataVersion>1</d:MetadataVersion></d:ProbeMatch>`,
		m.Endpoint, xmlEscape(m.Scopes), xmlEscape(m.XAddr),
	)
}

// Опрос устройств onvif
// TODO: Переделать на xml/go
func envelope(relatesTo, body string) string {
	return fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?><e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope" xmlns:w="http://schemas.xmlsoap.org/ws/2005/04/discovery" xmlns:a="http://www.w3.org/2005/08/addressing" xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery" xmlns:dn="http://www.onvif.org/ver10/network/wsdl"><e:Header><a:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/ProbeMatches</a:Action><a:MessageID>%s</a:MessageID><a:RelatesTo>%s</a:RelatesTo><a:To>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</a:To></e:Header><e:Body><d:ProbeMatches>%s</d:ProbeMatches></e:Body></e:Envelope>`,
		uuidURN(), relatesTo, body,
	)
}

// Проверка формата пакета
func looksLikeProbe(xml string) bool {
	low := strings.ToLower(xml)
	return strings.Contains(low, "/discovery/probe") || strings.Contains(xml, "<Probe") || strings.Contains(xml, ":Probe")
}

func extractTag(xml, tag string) string {
	open1, close1 := "<a:"+tag+">", "</a:"+tag+">"
	if i := strings.Index(xml, open1); i >= 0 {
		i += len(open1)
		if j := strings.Index(xml[i:], close1); j >= 0 {
			return xml[i : i+j]
		}
	}
	open2, close2 := "<"+tag+">", "</"+tag+">"
	if i := strings.Index(xml, open2); i >= 0 {
		i += len(open2)
		if j := strings.Index(xml[i:], close2); j >= 0 {
			return xml[i : i+j]
		}
	}
	return ""
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", `'`, "&apos;").Replace(s)
}

func uuidURN() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func guessLocalIP(raddr net.Addr) string {
	ra, _ := net.ResolveUDPAddr("udp4", raddr.String())
	conn, err := net.Dial("udp4", fmt.Sprintf("%s:%d", ra.IP.String(), ra.Port))
	if err == nil {
		defer conn.Close()
		if la, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			return la.IP.String()
		}
	}
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if (ifc.Flags & net.FlagUp) == 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok {
				ip := ipnet.IP.To4()
				if ip != nil && !ip.IsLoopback() {
					return ip.String()
				}
			}
		}
	}
	return "127.0.0.1"
}
