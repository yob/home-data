package amber

import (
	"context"
	"fmt"
	"strconv"
	"time"

	amberClient "github.com/yob/home-data/amber"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/core/memorystate"
	"github.com/yob/home-data/pubsub"
)

type Config struct {
	Token string
}

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state memorystate.StateReader, config Config) {
	publish := bus.PublishChannel()
	if config.Token == "" {
		logger.Fatal("amber: API key not found")
		return
	}

	client := amberClient.NewClient(config.Token)

	ctx := context.Background()
	sites, err := client.GetSites(ctx)
	if err != nil {
		logger.Fatal(fmt.Sprintf("amber: error fetching sites - %v", err))
		return
	}

	if len(sites) != 1 {
		logger.Fatal(fmt.Sprintf("amber: found %d sites, need 1", len(sites)))
		return
	}
	site := sites[0]

	for {
		time.Sleep(60 * time.Second)

		ctx := context.Background()
		prices, err := client.GetCurrentPrices(ctx, site)

		if err != nil {
			logger.Error(fmt.Sprintf("amber: error fetching prices - %v", err))
			continue
		}
		generalPrice := amberClient.Price{}
		feedInPrice := amberClient.Price{}

		for _, price := range prices {
			if price.ChannelType == "general" {
				generalPrice = price
			} else if price.ChannelType == "feedIn" {
				feedInPrice = price
			}
		}

		if generalPrice.Type != "" {
			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent("amber.general.cents_per_kwh", strconv.FormatFloat(generalPrice.PerKwh, 'f', -1, 64)),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent("amber.general.spot_cents_per_kwh", strconv.FormatFloat(generalPrice.SpotPerKwh, 'f', -1, 64)),
			}

			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent("amber.general.renewables", strconv.FormatFloat(generalPrice.Renewables, 'f', -1, 64)),
			}
		}
		if feedInPrice.Type != "" {
			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent("amber.feedin.cents_per_kwh", strconv.FormatFloat(feedInPrice.PerKwh, 'f', -1, 64)),
			}
		}
	}
}
