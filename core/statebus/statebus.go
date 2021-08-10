package statebus

import (
	"fmt"

	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/pubsub"
)

const (
	updateBufferSize = 10
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.State) {
	subStateUpdate, _ := bus.Subscribe("state:update")
	defer subStateUpdate.Close()

	subStateDelete, _ := bus.Subscribe("state:delete")
	defer subStateDelete.Close()

	for {
		select {
		case event := <-subStateUpdate.Ch:
			if event.Type == "key-value" {
				stateUpdate(logger, state, event.Key, event.Value)
			}
		case event := <-subStateDelete.Ch:
			if event.Type == "value" {
				stateDelete(logger, state, event.Value)
			}
		}
	}
}

func stateUpdate(logger *logging.Logger, state homestate.State, key string, value string) {
	state.Store(key, value)

	logger.Debug(fmt.Sprintf("set %s to %s", key, value))
}

func stateDelete(logger *logging.Logger, state homestate.State, key string) {
	state.Remove(key)

	logger.Debug(fmt.Sprintf("delete %s", key))
}
