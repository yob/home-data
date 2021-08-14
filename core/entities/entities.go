package entities

import (
	"fmt"
	"strconv"
	"time"

	"github.com/yob/home-data/pubsub"
)

type SensorBoolean struct {
	bus   *pubsub.Pubsub
	topic string
}

type SensorGauge struct {
	bus   *pubsub.Pubsub
	topic string
}

type SensorTime struct {
	bus   *pubsub.Pubsub
	topic string
}

func NewSensorBoolean(bus *pubsub.Pubsub, topic string) *SensorBoolean {
	return &SensorBoolean{
		bus:   bus,
		topic: topic,
	}
}

func (s *SensorBoolean) Update(value bool) {
	intValue := 0
	if value {
		intValue = 1
	}
	publish := s.bus.PublishChannel()
	publish <- pubsub.PubsubEvent{
		Topic: "state:update",
		Data:  pubsub.NewKeyValueEvent(s.topic, fmt.Sprintf("%d", intValue)),
	}
}

func (s *SensorBoolean) Unset() {
	publish := s.bus.PublishChannel()
	publish <- pubsub.PubsubEvent{
		Topic: "state:delete",
		Data:  pubsub.NewValueEvent(s.topic),
	}
}

func NewSensorGauge(bus *pubsub.Pubsub, topic string) *SensorGauge {
	return &SensorGauge{
		bus:   bus,
		topic: topic,
	}
}

func (s *SensorGauge) Update(value float64) {
	strValue := strconv.FormatFloat(value, 'f', 1, 64)
	publish := s.bus.PublishChannel()
	publish <- pubsub.PubsubEvent{
		Topic: "state:update",
		Data:  pubsub.NewKeyValueEvent(s.topic, strValue),
	}
}

func (s *SensorGauge) Unset() {
	publish := s.bus.PublishChannel()
	publish <- pubsub.PubsubEvent{
		Topic: "state:delete",
		Data:  pubsub.NewValueEvent(s.topic),
	}
}

func NewSensorTime(bus *pubsub.Pubsub, topic string) *SensorTime {
	return &SensorTime{
		bus:   bus,
		topic: topic,
	}
}

func (s *SensorTime) Update(value time.Time) {
	publish := s.bus.PublishChannel()
	publish <- pubsub.PubsubEvent{
		Topic: "state:update",
		Data:  pubsub.NewKeyValueEvent(s.topic, value.Format(time.RFC3339)),
	}
}

func (s *SensorTime) Unset() {
	publish := s.bus.PublishChannel()
	publish <- pubsub.PubsubEvent{
		Topic: "state:delete",
		Data:  pubsub.NewValueEvent(s.topic),
	}
}
