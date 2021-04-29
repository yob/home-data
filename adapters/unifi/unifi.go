package unifi

import (
	"fmt"
	"log"
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

func Poll(publish chan pubsub.PubsubEvent, config Config) {

	u, err := unifi.Login(config.UnifiUser, config.UnifiPass, config.Address, config.UnifiPort, config.UnifiSite, unifiApiVersion)
	if err != nil {
		log.Fatalf("Unifi login returned error: %v\n", err)
	}
	defer u.Logout()

	for {
		site, err := u.Site("default")
		if err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}
		stations, err := u.Sta(site)
		if err != nil {
			log.Fatalf("ERROR: %v\n", err)
		}

		for _, s := range stations {
			if stationName, ok := config.IpMap[s.IP]; ok {
				lastSeen := time.Unix(s.LastSeen, 0).UTC()
				publish <- pubsub.PubsubEvent{
					Topic: "state:update",
					Data:  pubsub.KeyValueData{Key: fmt.Sprintf("unifi.presence.last_seen.%s", stationName), Value: lastSeen.Format(time.RFC3339)},
				}
			}
		}

		time.Sleep(20 * time.Second)
	}
}
