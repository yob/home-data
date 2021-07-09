package statebus

import (
	"fmt"
	"sync"

	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state *sync.Map) {
	subStateUpdate, _ := bus.Subscribe("state:update")
	defer subStateUpdate.Close()

	for event := range subStateUpdate.Ch {
		if event.Type != "key-value" {
			continue
		}

		stateUpdate(logger, state, event.Key, event.Value)
	}
}

func stateUpdate(logger *logging.Logger, state *sync.Map, property string, value string) {
	existingValue, ok := state.Load(property)

	// if the property doesn't exist in the state yet, or it exists with a different value, then update it
	if !ok || existingValue != value {
		state.Store(property, value)
		logger.Debug(fmt.Sprintf("set %s to %s", property, value))
	}
}
