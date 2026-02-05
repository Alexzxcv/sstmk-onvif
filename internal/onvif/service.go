package onvif

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type logRW struct {
	http.ResponseWriter
	status int
	buf    strings.Builder
}

func (lw *logRW) WriteHeader(code int) {
	lw.status = code
	lw.ResponseWriter.WriteHeader(code)
}

func (lw *logRW) Write(p []byte) (int, error) {
	if lw.buf.Len() < 500 { // первые 500 символов
		remain := 500 - lw.buf.Len()
		if len(p) > remain {
			lw.buf.Write(p[:remain])
		} else {
			lw.buf.Write(p)
		}
	}
	return lw.ResponseWriter.Write(p)
}

type EventService struct {
	subscriptionManager *SubscriptionManager
	baseURL             string
}

func NewEventService(baseURL string) *EventService {
	return &EventService{
		subscriptionManager: NewSubscriptionManager(),
		baseURL:             baseURL,
	}
}

func (es *EventService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	lw := &logRW{ResponseWriter: w, status: 200}
	w = lw
	defer func() {
		log.Printf("[ONVIF] RESP %s %s status=%d CT=%q head=%q",
			r.Method, r.URL.Path, lw.status, w.Header().Get("Content-Type"), lw.buf.String())
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	b, _ := io.ReadAll(r.Body)
	bodyStr := string(b)

	action := extractAction(bodyStr)

	// if strings.Contains(bodyStr, "GetServiceCapabilities") {
	// 	es.handleGetServiceCapabilities(w, r, bodyStr)
	// } else if strings.Contains(bodyStr, "CreatePullPointSubscription") {
	// 	es.handleCreatePullPointSubscription(w, r)
	// } else if strings.Contains(bodyStr, "PullMessages") {
	// 	es.handlePullMessages(w, r, bodyStr)
	// } else {
	// 	log.Printf("[ONVIF] Unknown SOAP action in body: %s", bodyStr[:min(200, len(bodyStr))])
	// 	http.Error(w, "Unknown SOAP action", http.StatusBadRequest)
	// }

	switch {
	case strings.Contains(action, "GetServiceCapabilitiesRequest") || strings.Contains(bodyStr, "GetServiceCapabilities"):
		es.handleGetServiceCapabilities(w, r, bodyStr)
	case strings.Contains(action, "CreatePullPointSubscriptionRequest") || strings.Contains(bodyStr, "CreatePullPointSubscription"):
		es.handleCreatePullPointSubscription(w, r, bodyStr)
	case strings.Contains(action, "PullMessagesRequest") || strings.Contains(bodyStr, "PullMessages"):
		es.handlePullMessages(w, r, bodyStr)
	default:
		log.Printf("[ONVIF] Unknown SOAP action in body: %s", bodyStr[:min(200, len(bodyStr))])
		http.Error(w, "Unknown SOAP action", http.StatusBadRequest)
	}
}

func extractAction(soap string) string {
	// очень простой парсер Action
	start := strings.Index(soap, "<a:Action")
	if start == -1 {
		return ""
	}
	start = strings.Index(soap[start:], ">")
	if start == -1 {
		return ""
	}
	// start сейчас относительный, пересчитаем
	startAbs := strings.Index(soap, "<a:Action")
	startAbs = startAbs + strings.Index(soap[startAbs:], ">") + 1

	endAbs := strings.Index(soap[startAbs:], "</a:Action>")
	if endAbs == -1 {
		return ""
	}
	return strings.TrimSpace(soap[startAbs : startAbs+endAbs])
}

func normalizeWSAValue(s string) string {
	// убираем CR/LF и лишние пробелы внутри, чтобы "uuid\n:xxx" стало "uuid:xxx"
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.TrimSpace(s)
	// иногда после удаления \n остаётся "uuid :xxx"
	s = strings.ReplaceAll(s, "uuid :", "uuid:")
	s = strings.ReplaceAll(s, "uuid: ", "uuid:")
	return s
}

func writeSOAP12(w http.ResponseWriter, headerXML, bodyXML string) {
	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	envelope := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<env:Envelope xmlns:env="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://www.w3.org/2005/08/addressing">` +
		`<env:Header>` + headerXML + `</env:Header>` +
		`<env:Body>` + bodyXML + `</env:Body>` +
		`</env:Envelope>`

	log.Printf("[ONVIF] RESP CT=%q len=%d head=%q",
		w.Header().Get("Content-Type"),
		len(envelope),
		envelope[:min(300, len(envelope))],
	)

	_, _ = w.Write([]byte(envelope))
}

func (es *EventService) handleCreatePullPointSubscription(w http.ResponseWriter, r *http.Request, reqBody string) {
	reqMsgID := extractMessageIDFromBody(reqBody)

	action := "http://www.onvif.org/ver10/events/wsdl/EventPortType/CreatePullPointSubscriptionResponse"
	header := soapHeader(action, reqMsgID)

	subID := uuid.New().String()
	es.subscriptionManager.CreateSubscription(subID, 24*time.Hour)

	addr := fmt.Sprintf("%s/onvif/events/subscription/%s", es.baseURL, subID)
	nowUTC := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	termUTC := time.Now().Add(24 * time.Hour).UTC().Format("2006-01-02T15:04:05.000Z")

	body := fmt.Sprintf(`<tev:CreatePullPointSubscriptionResponse
		xmlns:tev="http://www.onvif.org/ver10/events/wsdl"
		xmlns:wsa="http://www.w3.org/2005/08/addressing">
		<tev:SubscriptionReference>
			<wsa:Address>%s</wsa:Address>
		</tev:SubscriptionReference>
		<tev:CurrentTime>%s</tev:CurrentTime>
		<tev:TerminationTime>%s</tev:TerminationTime>
	</tev:CreatePullPointSubscriptionResponse>`,
		addr, nowUTC, termUTC)

	writeSOAP12(w, header, body)
	log.Printf("[ONVIF] PullPoint subscription created: %s", subID)
}

func (es *EventService) handlePullMessages(w http.ResponseWriter, r *http.Request, body string) {
	reqMsgID := extractMessageIDFromBody(body)

	action := "http://www.onvif.org/ver10/events/wsdl/PullPointSubscription/PullMessagesResponse"
	header := soapHeader(action, reqMsgID)

	subID := extractSubscriptionID(r.URL.Path)
	if subID == "" {
		subID = es.subscriptionManager.AnyActiveSubscriptionID()
	}
	if subID == "" {
		http.Error(w, "No active subscription", http.StatusBadRequest)
		return
	}

	sub := es.subscriptionManager.GetSubscription(subID)
	if sub == nil {
		http.Error(w, "Subscription not found", http.StatusNotFound)
		return
	}

	limit := extractMessageLimit(body)
	if limit == 0 {
		limit = 10
	}

	messages := sub.PullMessages(limit)
	nowUTC := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	termUTC := sub.TerminationTime.UTC().Format("2006-01-02T15:04:05.000Z")

	var notificationsXML strings.Builder
	for _, msg := range messages {
		// Извлекаем данные
		pictureB64 := msg.Data.SimpleItems[0].Value
		account := msg.Data.SimpleItems[1].Value
		category := "0" // По умолчанию
		mesures := ""   // Пустая строка по умолчанию

		eventMessage := fmt.Sprintf(`<tt:Message UtcTime="%s" PropertyOperation="Initialized"
			xmlns:tt="http://www.onvif.org/ver10/schema">
			<tt:Source>
				<tt:SimpleItem Name="Id" Value="%s"/>
			</tt:Source>
			<tt:Key/>
			<tt:Data>
				<tt:SimpleItem Name="Picture" Value="%s"/>
				<tt:SimpleItem Name="Category" Value="%s"/>
				<tt:SimpleItem Name="Mesures" Value="%s"/>
				<tt:SimpleItem Name="Account" Value="%s"/>
			</tt:Data>
		</tt:Message>`,
			msg.UtcTime,
			msg.Source.SimpleItem.Value,
			pictureB64,
			category,
			mesures,
			account)

		notificationsXML.WriteString(fmt.Sprintf(`<wsnt:NotificationMessage>
			<wsnt:Topic Dialect="http://www.onvif.org/ver10/tev/topicExpression/ConcreteSet"
			 xmlns:tns1="http://www.onvif.org/ver10/topics"
			 xmlns:tmk="http://www.inforion.ru/schemas/sstmk/onvif/topics/sensors">tmk:MetalDetector/tmk:Detect</wsnt:Topic>
			%s
		</wsnt:NotificationMessage>`, eventMessage))
	}

	body = fmt.Sprintf(`<tev:PullMessagesResponse xmlns:tev="http://www.onvif.org/ver10/events/wsdl"
		xmlns:wsnt="http://docs.oasis-open.org/wsn/b-2">
		<tev:CurrentTime>%s</tev:CurrentTime>
		<tev:TerminationTime>%s</tev:TerminationTime>
		%s
	</tev:PullMessagesResponse>`,
		nowUTC, termUTC, notificationsXML.String())

	writeSOAP12(w, header, body)
	log.Printf("[ONVIF] PullMessages: subscription=%s, messages=%d", subID, len(messages))
}

func soapHeader(action, relatesTo string) string {
	// новый MessageID для ответа
	msgID := "uuid:" + uuid.New().String()
	relatesTo = normalizeWSAValue(relatesTo)

	return fmt.Sprintf(
		`<a:Action env:mustUnderstand="1" xmlns:env="http://www.w3.org/2003/05/soap-envelope">%s</a:Action>`+
			`<a:MessageID>%s</a:MessageID>`+
			`<a:RelatesTo>%s</a:RelatesTo>`+
			`<a:To env:mustUnderstand="1" xmlns:env="http://www.w3.org/2003/05/soap-envelope">http://www.w3.org/2005/08/addressing/anonymous</a:To>`,
		action, msgID, relatesTo,
	)
}

func (es *EventService) handleGetServiceCapabilities(w http.ResponseWriter, r *http.Request, bodyStr string) {
	reqMsgID := extractMessageIDFromBody(bodyStr)

	// Action для ответа (у ONVIF часто именно "...GetServiceCapabilitiesResponse")
	action := "http://www.onvif.org/ver10/events/wsdl/EventPortType/GetServiceCapabilitiesResponse"
	header := soapHeader(action, reqMsgID)

	// Важно: Capabilities с флагами, чтобы совпадало с Device GetCapabilities.Events.*
	body := `<tev:GetServiceCapabilitiesResponse xmlns:tev="http://www.onvif.org/ver10/events/wsdl">
		<tev:Capabilities WSSubscriptionPolicySupport="false"
		                 WSPullPointSupport="true"
		                 WSPausableSubscriptionManagerInterfaceSupport="false"/>
	</tev:GetServiceCapabilitiesResponse>`

	writeSOAP12(w, header, body)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (es *EventService) PublishEvent(deviceID, image, account string) {
	msg := NewMetalDetectorMessage(deviceID, image, account)
	es.subscriptionManager.BroadcastMessage(msg)
	log.Printf("[ONVIF] Event published for device %s", deviceID)
}

func formatMessage(msg *Message) string {
	return fmt.Sprintf(`<tt:Message UtcTime="%s" PropertyOperation="%s" xmlns:tt="http://www.onvif.org/ver10/schema">
			<tt:Source>
				<tt:SimpleItem Name="%s" Value="%s" />
			</tt:Source>
			<tt:Key />
			<tt:Data>
				<tt:SimpleItem Name="%s" Value="%s" />
				<tt:SimpleItem Name="%s" Value="%s" />
			</tt:Data>
		</tt:Message>`,
		msg.UtcTime, msg.PropertyOperation,
		msg.Source.SimpleItem.Name, msg.Source.SimpleItem.Value,
		msg.Data.SimpleItems[0].Name, msg.Data.SimpleItems[0].Value,
		msg.Data.SimpleItems[1].Name, msg.Data.SimpleItems[1].Value)
}

func extractSubscriptionID(path string) string {
	parts := splitPath(path)
	// ожидаем "/subscription/{id}"
	if len(parts) >= 2 && parts[len(parts)-2] == "subscription" {
		return parts[len(parts)-1]
	}
	return ""
}

func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func generateUUID() string {
	return uuid.New().String()
}

func extractMessageIDFromBody(bodyStr string) string {
	// у тебя была эта функция, но она ищет "<a:MessageID>".
	// В логах MessageID может быть с переносом строки, поэтому делаем чуть устойчивее:
	start := strings.Index(bodyStr, "<a:MessageID")
	if start == -1 {
		return "uuid:" + uuid.New().String()
	}
	start = strings.Index(bodyStr[start:], ">")
	if start == -1 {
		return "uuid:" + uuid.New().String()
	}
	// start относительный — пересчёт
	startAbs := strings.Index(bodyStr, "<a:MessageID")
	startAbs = startAbs + strings.Index(bodyStr[startAbs:], ">") + 1

	endAbs := strings.Index(bodyStr[startAbs:], "</a:MessageID>")
	if endAbs == -1 {
		return "uuid:" + uuid.New().String()
	}
	return normalizeWSAValue(bodyStr[startAbs : startAbs+endAbs])
	// return strings.TrimSpace(bodyStr[startAbs : startAbs+endAbs])
}

func extractMessageLimit(body string) int {
	// Simple extraction of MessageLimit from SOAP body
	if strings.Contains(body, "MessageLimit") {
		// This is a simplified implementation
		return 10
	}
	return 10
}
