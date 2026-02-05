package onvif

import (
	"encoding/xml"
	"time"
)

type Message struct {
	XMLName           xml.Name `xml:"http://www.onvif.org/ver10/schema Message"`
	UtcTime           string   `xml:"UtcTime,attr"`
	PropertyOperation string   `xml:"PropertyOperation,attr"`
	Source            Source   `xml:"Source"`
	Key               Key      `xml:"Key"`
	Data              Data     `xml:"Data"`
}

type Source struct {
	SimpleItem SimpleItem `xml:"SimpleItem"`
}

type Key struct{}

type Data struct {
	SimpleItems []SimpleItem `xml:"SimpleItem"`
}

type SimpleItem struct {
	Name  string `xml:"Name,attr"`
	Value string `xml:"Value,attr"`
}

type NotificationMessage struct {
	XMLName xml.Name `xml:"http://docs.oasis-open.org/wsn/b-2 NotificationMessage"`
	Topic   Topic    `xml:"Topic"`
	Message Message  `xml:"Message"`
}

type Topic struct {
	XMLName xml.Name `xml:"http://docs.oasis-open.org/wsn/b-2 Topic"`
	Dialect string   `xml:"Dialect,attr"`
	Value   string   `xml:",chardata"`
}

type PullMessagesResponse struct {
	XMLName              xml.Name              `xml:"http://www.onvif.org/ver10/events/wsdl PullMessagesResponse"`
	CurrentTime          string                `xml:"CurrentTime"`
	TerminationTime      string                `xml:"TerminationTime"`
	NotificationMessages []NotificationMessage `xml:"NotificationMessage"`
}

type CreatePullPointSubscriptionResponse struct {
	XMLName               xml.Name  `xml:"http://www.onvif.org/ver10/events/wsdl CreatePullPointSubscriptionResponse"`
	SubscriptionReference Reference `xml:"SubscriptionReference"`
	CurrentTime           string    `xml:"CurrentTime"`
	TerminationTime       string    `xml:"TerminationTime"`
}

type Reference struct {
	XMLName xml.Name `xml:"http://www.w3.org/2005/08/addressing EndpointReference"`
	Address string   `xml:"Address"`
}

func NewMetalDetectorMessage(deviceID, image, account string) *Message {
	return &Message{
		UtcTime:           time.Now().UTC().Format("2006-01-02T15:04:05.0000000Z"),
		PropertyOperation: "Initialized",
		Source: Source{
			SimpleItem: SimpleItem{Name: "Id", Value: deviceID},
		},
		Key: Key{},
		Data: Data{
			SimpleItems: []SimpleItem{
				{Name: "Picture", Value: image},
				{Name: "Account", Value: account},
			},
		},
	}
}
