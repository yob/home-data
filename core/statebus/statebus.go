package statebus

import (
	"fmt"

	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/core/memorystate"
	"github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state *memorystate.State) {
	subStateUpdate, _ := bus.Subscribe("state:update")
	defer subStateUpdate.Close()

	for event := range subStateUpdate.Ch {
		if event.Type != "key-value" {
			continue
		}

		stateUpdate(logger, state, event.Key, event.Value)
	}
}

func stateUpdate(logger *logging.Logger, state *memorystate.State, property string, value string) {
	existingValue, ok := state.Read(property)

	// if the property doesn't exist in the state yet, or it exists with a different value, then update it
	if !ok || existingValue != value {
		state.Store(property, value)
		logger.Debug(fmt.Sprintf("set %s to %s", property, value))
	}
}
