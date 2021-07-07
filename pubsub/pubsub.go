package pubsub

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	channelBufferSize = 100
)

type Pubsub struct {
	mu              sync.RWMutex
	subs            map[string][]*Subscription
	publishChannel  chan PubsubEvent
	closeSubChannel chan string
	closed          bool
}

type Subscription struct {
	Topic   string
	Ch      chan EventData
	uuid    string
	closeCh chan string
}

type PubsubEvent struct {
	Topic string
	Data  EventData
}

type HttpRequest struct {
	Headers map[string]string
	Body    string
}

type HttpResponse struct {
	Status int
	Body   string
}

type EventData struct {
	Type         string
	Key          string
	Value        string
	HttpRequest  HttpRequest
	HttpResponse HttpResponse
}

func NewValueEvent(value string) EventData {
	return EventData{
		Type:  "value",
		Value: value,
	}
}

func NewKeyValueEvent(key string, value string) EventData {
	return EventData{
		Type:  "key-value",
		Key:   key,
		Value: value,
	}
}

// we'll probably need to capture the request headers here at some point, but I
// don't need them yet
func NewHttpRequestEvent(body string, uuid string) EventData {
	return EventData{
		Type: "http-request",
		Key:  uuid,
		HttpRequest: HttpRequest{
			Body: body,
		},
	}
}

func NewHttpResponseEvent(status int, body string, uuid string) EventData {
	return EventData{
		Type: "http-response",
		Key:  uuid,
		HttpResponse: HttpResponse{
			Status: status,
			Body:   body,
		},
	}
}

func NewPubsub() *Pubsub {
	ps := &Pubsub{}
	ps.subs = make(map[string][]*Subscription)
	ps.publishChannel = make(chan PubsubEvent, channelBufferSize)
	ps.closeSubChannel = make(chan string, channelBufferSize)

	// All subscriptions we hand out include a Close() method that will signal
	// back to us when the receiver isn't using it any more. The signal arrives on
	// a shared channel for the entire bus, so when it arrives walk through the
	// live subscriptions to find and remove it
	//
	// The current algorithm is super inefficient, but it works. I'll optimise it
	// when the ineffeciency is a problem.
	go func() {
		for {
			select {
			case uuidToRemove := <-ps.closeSubChannel:
				ps.mu.Lock()
				for topic, subs := range ps.subs {
					for idx, sub := range subs {
						if sub.uuid == uuidToRemove {
							// re-slice to remove the Subscription we don't need any more
							ps.subs[topic][idx] = ps.subs[topic][len(ps.subs[topic])-1]
							ps.subs[topic] = ps.subs[topic][:len(ps.subs[topic])-1]
							continue
						}
					}

					// this topic is totally unused now, so we don't need to keep it around
					if len(ps.subs[topic]) == 0 {
						delete(ps.subs, topic)
					}
				}
				ps.mu.Unlock()
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()

	return ps
}

// Return a subscription struct that can be used to receive events. It's critical that
// the subscription is closed when it's not needed any more. A typical pattern looks
// like this:
//
//   subEveryMinute, _ := bus.Subscribe("every:minute")
//   defer subEveryMinute.Close()
//   for event := range subEveryMinute.Ch {
//     // do things with event
//   }
//
func (ps *Pubsub) Subscribe(topic string) (*Subscription, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	subUUID, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	sub := &Subscription{
		Topic:   topic,
		Ch:      make(chan EventData, channelBufferSize),
		uuid:    subUUID.String(),
		closeCh: ps.closeSubChannel,
	}
	ps.subs[topic] = append(ps.subs[topic], sub)
	return sub, nil
}

func (ps *Pubsub) PublishChannel() chan PubsubEvent {
	return ps.publishChannel
}

func (ps *Pubsub) Run() {

	for {
		select {
		case event := <-ps.publishChannel:
			if ps.closed {
				continue
			}
			ps.mu.RLock()
			for _, sub := range ps.subs[event.Topic] {
				sub.Ch <- event.Data
			}
			ps.mu.RUnlock()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (ps *Pubsub) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !ps.closed {
		ps.closed = true
		for _, subs := range ps.subs {
			for _, sub := range subs {
				close(sub.Ch)
			}
		}
	}
}

func (s *Subscription) Close() {
	s.closeCh <- s.uuid
}
