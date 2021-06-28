package amber

import (
	"context"
	"fmt"
	"strconv"
	"time"

	amberClient "github.com/yob/home-data/amber"
	"github.com/yob/home-data/pubsub"
)

func Poll(bus *pubsub.Pubsub, apiKey string) {
	publish := bus.PublishChannel()
	if apiKey == "" {
		errorLog(publish, "amber: API key not found")
		return
	}

	client := amberClient.NewClient(apiKey)

	ctx := context.Background()
	sites, err := client.GetSites(ctx)
	if err != nil {
		errorLog(publish, fmt.Sprintf("amber: error fetching sites - %v", err))
		return
	}

	if len(sites) != 1 {
		errorLog(publish, fmt.Sprintf("amber: found %d sites, need 1", len(sites)))
		return
	}
	site := sites[0]

	for {
		time.Sleep(60 * time.Second)

		ctx := context.Background()
		prices, err := client.GetCurrentPrices(ctx, site)

		if err != nil {
			errorLog(publish, fmt.Sprintf("amber: error fetching prices - %v", err))
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
				Data:  pubsub.KeyValueData{Key: "amber.general.cents_per_kwh", Value: strconv.FormatFloat(generalPrice.PerKwh, 'f', -1, 64)},
			}

			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.KeyValueData{Key: "amber.general.spot_cents_per_kwh", Value: strconv.FormatFloat(generalPrice.SpotPerKwh, 'f', -1, 64)},
			}

			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.KeyValueData{Key: "amber.general.renewables", Value: strconv.FormatFloat(generalPrice.Renewables, 'f', -1, 64)},
			}
		}
		if feedInPrice.Type != "" {
			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.KeyValueData{Key: "amber.feedin.cents_per_kwh", Value: strconv.FormatFloat(feedInPrice.PerKwh, 'f', -1, 64)},
			}
		}
	}
}

func debugLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "DEBUG", Value: message},
	}

}

func errorLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.KeyValueData{Key: "ERROR", Value: message},
	}

}