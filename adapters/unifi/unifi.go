package unifi

import (
	"fmt"
	"log"
	"time"

	pubsub "github.com/yob/home-data/pubsub"

	"github.com/dim13/unifi"
)

func Poll(publish chan pubsub.PubsubEvent, address string, unifi_user string, unifi_pass string, unifi_port string, unifi_site string) {

	// TODO pass this is as config
	var ipMap = map[string]string{
		"10.1.1.123": "james",
		"10.1.1.134": "andrea",
	}

	u, err := unifi.Login(unifi_user, unifi_pass, address, unifi_port, unifi_site, 5)
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
			if stationName, ok := ipMap[s.IP]; ok {
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
