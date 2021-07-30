package unifi

import (
	"fmt"
	"time"

	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/logging"
	pubsub "github.com/yob/home-data/pubsub"

	"github.com/dim13/unifi"
)

var (
	unifiApiVersion = 5
)

type configData struct {
	address   string
	unifiUser string
	unifiPass string
	unifiPort string
	unifiSite string
	ipMap     map[string]string
}

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, configSection *conf.ConfigSection) {
	publish := bus.PublishChannel()

	config, err := newConfigFromSection(configSection)
	if err != nil {
		logger.Fatal(fmt.Sprintf("unifi: %v", err))
		return
	}

	u, err := unifi.Login(config.unifiUser, config.unifiPass, config.address, config.unifiPort, config.unifiSite, unifiApiVersion)
	if err != nil {
		logger.Fatal(fmt.Sprintf("unifi: login returned error: %v", err))
		return
	}
	defer u.Logout()

	for {
		site, err := u.Site(config.unifiSite)
		if err != nil {
			logger.Fatal(fmt.Sprintf("unifi: %v", err))
			return
		}
		stations, err := u.Sta(site)
		if err != nil {
			logger.Fatal(fmt.Sprintf("unifi: %v", err))
			return
		}

		for _, s := range stations {
			if stationName, ok := config.ipMap[s.IP]; ok {
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

func newConfigFromSection(configSection *conf.ConfigSection) (configData, error) {
	address, err := configSection.GetString("address")
	if err != nil {
		return configData{}, fmt.Errorf("address not found in config")
	}

	user, err := configSection.GetString("user")
	if err != nil {
		return configData{}, fmt.Errorf("user not found in config")
	}

	pass, err := configSection.GetString("pass")
	if err != nil {
		return configData{}, fmt.Errorf("pass not found in config")
	}

	port, err := configSection.GetString("port")
	if err != nil {
		return configData{}, fmt.Errorf("port not found in config")
	}

	site, err := configSection.GetString("site")
	if err != nil {
		return configData{}, fmt.Errorf("site not found in config")
	}

	names, err := configSection.GetStringMap("names")
	if err != nil {
		return configData{}, fmt.Errorf("names not found in config")
	}

	return configData{
		address:   address,
		unifiUser: user,
		unifiPass: pass,
		unifiPort: port,
		unifiSite: site,
		ipMap:     names,
	}, nil
}
