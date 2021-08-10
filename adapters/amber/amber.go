package amber

import (
	"context"
	"fmt"
	"strconv"
	"time"

	amberClient "github.com/yob/go-amber"
	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/pubsub"
)

const (
	// we consider prices below this cheap and will set some state so other parts of the system
	// can choose to respond
	cheapImportThreshold = 18
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, config *conf.ConfigSection) {
	publish := bus.PublishChannel()

	apiToken, err := config.GetString("api_key")
	if err != nil {
		logger.Fatal("amber: api_key not found in config")
		return
	}

	client := amberClient.NewClient(apiToken)

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

		// return next 12 hours of general forecasts. They'll be returned in sorted order.
		prices, err = client.GetForecastGeneralPrices(ctx, site)

		if err != nil {
			logger.Error(fmt.Sprintf("amber: error fetching forecast prices - %v", err))
			continue
		}

		// TODO would it be better to store (a) cheapest price in the next 12 hours and (b) minutes until the cheapest price in
		//      the next 12 hours?
		foundCheapPrice := false

		for _, price := range prices {
			if price.PerKwh <= cheapImportThreshold && !foundCheapPrice {
				foundCheapPrice = true
				publish <- pubsub.PubsubEvent{
					Topic: "state:update",
					Data:  pubsub.NewKeyValueEvent("amber.general.cheap_imports_at", price.StartTime.Format(time.RFC3339)),
				}

				publish <- pubsub.PubsubEvent{
					Topic: "state:update",
					Data:  pubsub.NewKeyValueEvent("amber.general.cheap_import_price", strconv.FormatFloat(price.PerKwh, 'f', -1, 64)),
				}

				minsUntilCheapImports := int64(price.StartTime.Sub(time.Now()) / time.Minute)
				publish <- pubsub.PubsubEvent{
					Topic: "state:update",
					Data:  pubsub.NewKeyValueEvent("amber.general.mins_until_cheap_imports", strconv.FormatInt(minsUntilCheapImports, 10)),
				}
			}
		}

		if !foundCheapPrice {
			publish <- pubsub.PubsubEvent{
				Topic: "state:delete",
				Data:  pubsub.NewValueEvent("amber.general.cheap_imports_at"),
			}
			publish <- pubsub.PubsubEvent{
				Topic: "state:delete",
				Data:  pubsub.NewValueEvent("amber.general.cheap_import_price"),
			}
			publish <- pubsub.PubsubEvent{
				Topic: "state:delete",
				Data:  pubsub.NewValueEvent("amber.general.mins_until_cheap_imports"),
			}

		}
	}
}
