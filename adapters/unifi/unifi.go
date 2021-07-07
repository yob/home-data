package unifi

import (
	"fmt"
	"time"

	pubsub "github.com/yob/home-data/pubsub"

	"github.com/dim13/unifi"
)

var (
	unifiApiVersion = 5
)

type Config struct {
	Address   string
	UnifiUser string
	UnifiPass string
	UnifiPort string
	UnifiSite string
	IpMap     map[string]string
}

func Init(bus *pubsub.Pubsub, config Config) {
	publish := bus.PublishChannel()

	u, err := unifi.Login(config.UnifiUser, config.UnifiPass, config.Address, config.UnifiPort, config.UnifiSite, unifiApiVersion)
	if err != nil {
		fatalLog(publish, fmt.Sprintf("Unifi login returned error: %v", err))
		return
	}
	defer u.Logout()

	for {
		site, err := u.Site("default")
		if err != nil {
			fatalLog(publish, fmt.Sprintf("%v", err))
			return
		}
		stations, err := u.Sta(site)
		if err != nil {
			fatalLog(publish, fmt.Sprintf("%v", err))
			return
		}

		for _, s := range stations {
			if stationName, ok := config.IpMap[s.IP]; ok {
				lastSeen := time.Unix(s.LastSeen, 0).UTC()
				publish <- pubsub.PubsubEvent{
					Topic: "state:update",
					Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("unifi.presence.last_seen.%s", stationName), lastSeen.Format(time.RFC3339)),
				}
			}
		}

		time.Sleep(20 * time.Second)
	}
}

func fatalLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.NewKeyValueEvent("FATAL", message),
	}

}
