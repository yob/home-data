package timers

import (
	"time"

	"github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub) {
	everyMinuteEvent(bus.PublishChannel())
}

func everyMinuteEvent(publish chan pubsub.PubsubEvent) {
	lastBroadcast := time.Now()

	for {
		if time.Now().After(lastBroadcast.Add(time.Second * 60)) {
			publish <- pubsub.PubsubEvent{
				Topic: "every:minute",
				Data:  pubsub.NewValueEvent(time.Now().Format(time.RFC3339)),
			}
			lastBroadcast = time.Now()
		}
		time.Sleep(1 * time.Second)
	}
}
