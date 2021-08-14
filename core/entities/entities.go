package entities

import (
	"fmt"
	"strconv"

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
