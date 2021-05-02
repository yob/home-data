package statebus

import (
	"fmt"
	"sync"

	"github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub, state *sync.Map) {
	ch_state_update := bus.Subscribe("state:update")
	ch_publish := bus.PublishChannel()
	for elem := range ch_state_update {
		stateUpdate(ch_publish, state, elem.Key, elem.Value)
	}
}

func stateUpdate(publish chan pubsub.PubsubEvent, state *sync.Map, property string, value string) {
	existingValue, ok := state.Load(property)

	// if the property doesn't exist in the state yet, or it exists with a different value, then update it
	if !ok || existingValue != value {
		state.Store(property, value)
		debugLog(publish, fmt.Sprintf("set %s to %s", property, value))
	}
}

func debugLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "DEBUG", Value: message},
	}

}
