package onvif

import (
	"sync"
	"time"
)

type Subscription struct {
	ID              string
	Messages        []*Message
	TerminationTime time.Time
	mu              sync.Mutex
}

func (s *Subscription) AddMessage(msg *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, msg)
}

func (s *Subscription) PullMessages(limit int) []*Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Messages) == 0 {
		return nil
	}

	count := limit
	if count > len(s.Messages) {
		count = len(s.Messages)
	}

	result := make([]*Message, count)
	copy(result, s.Messages[:count])
	s.Messages = s.Messages[count:]

	return result
}

type SubscriptionManager struct {
	subscriptions map[string]*Subscription
	mu            sync.RWMutex
}

func NewSubscriptionManager() *SubscriptionManager {
	return &SubscriptionManager{
		subscriptions: make(map[string]*Subscription),
	}
}

func (sm *SubscriptionManager) CreateSubscription(id string, ttl time.Duration) *Subscription {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sub := &Subscription{
		ID:              id,
		Messages:        make([]*Message, 0),
		TerminationTime: time.Now().Add(ttl),
	}

	sm.subscriptions[id] = sub
	return sub
}

func (sm *SubscriptionManager) GetSubscription(id string) *Subscription {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.subscriptions[id]
}

func (sm *SubscriptionManager) BroadcastMessage(msg *Message) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, sub := range sm.subscriptions {
		if time.Now().Before(sub.TerminationTime) {
			sub.AddMessage(msg)
		}
	}
}

func (sm *SubscriptionManager) AnyActiveSubscriptionID() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for id, sub := range sm.subscriptions {
		if time.Now().Before(sub.TerminationTime) {
			return id
		}
	}
	return ""
}
