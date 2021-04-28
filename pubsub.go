package main

import (
	"sync"
	"time"
)

const (
	channelBufferSize = 100
)

type Pubsub struct {
	mu             sync.RWMutex
	subs           map[string][]chan KeyValueData
	publishChannel chan PubsubEvent
	closed         bool
}

type PubsubEvent struct {
	topic string
	data  KeyValueData
}

type KeyValueData struct {
	key   string
	value string
}

func NewPubsub() *Pubsub {
	ps := &Pubsub{}
	ps.subs = make(map[string][]chan KeyValueData)
	ps.publishChannel = make(chan PubsubEvent, channelBufferSize)
	return ps
}

func (ps *Pubsub) Subscribe(topic string) <-chan KeyValueData {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ch := make(chan KeyValueData, channelBufferSize)
	ps.subs[topic] = append(ps.subs[topic], ch)
	return ch
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
			for _, ch := range ps.subs[event.topic] {
				ch <- event.data
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
			for _, ch := range subs {
				close(ch)
			}
		}
	}
}
