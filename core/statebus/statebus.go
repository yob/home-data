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

	bufferedUpdates := make(map[string]string, updateBufferSize)

	for event := range subStateUpdate.Ch {
		if event.Type != "key-value" {
			continue
		}
		bufferedUpdates[event.Key] = event.Value

		if len(subStateUpdate.Ch) == 0 || len(bufferedUpdates) == updateBufferSize {
			stateUpdate(logger, state, bufferedUpdates)
			bufferedUpdates = make(map[string]string, updateBufferSize)
		}
	}
}

func stateUpdate(logger *logging.Logger, state homestate.State, updates map[string]string) {
	state.StoreMulti(updates)

	for k, v := range updates {
		logger.Debug(fmt.Sprintf("set %s to %s", k, v))
	}
}
