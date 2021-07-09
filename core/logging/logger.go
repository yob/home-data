package logging

import (
	"github.com/yob/home-data/pubsub"
)

type Logger struct {
	bus *pubsub.Pubsub
}

func NewLogger(bus *pubsub.Pubsub) *Logger {
	return &Logger{
		bus: bus,
	}
}

func (logger *Logger) Debug(message string) {
	logger.bus.PublishChannel() <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.NewKeyValueEvent("DEBUG", message),
	}
}

func (logger *Logger) Error(message string) {
	logger.bus.PublishChannel() <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.NewKeyValueEvent("ERROR", message),
	}
}

func (logger *Logger) Fatal(message string) {
	logger.bus.PublishChannel() <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.NewKeyValueEvent("FATAL", message),
	}
}
