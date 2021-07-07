package logging

import (
	"fmt"

	"github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub) {
	subLog, _ := bus.Subscribe("log:new")
	defer subLog.Close()

	for event := range subLog.Ch {
		fmt.Printf("%s: %s\n", event.Key, event.Value)
	}
}
