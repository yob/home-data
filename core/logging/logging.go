package logging

import (
	"fmt"

	"github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub) {
	ch_log := bus.Subscribe("log:new")
	for event := range ch_log {
		fmt.Printf("%s: %s\n", event.Key, event.Value)
	}
}
