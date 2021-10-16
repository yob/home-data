package amber

import (
	"context"
	"fmt"
	"time"

	amberClient "github.com/yob/go-amber"
	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/entities"
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
	apiToken, err := config.GetString("api_key")
	if err != nil {
		logger.Fatal("amber: api_key not found in config")
		return
	}

	for {
		time.Sleep(30 * time.Second)

		client := amberClient.NewClient(apiToken)

		ctx := context.Background()
		sites, err := client.GetSites(ctx)
		if err != nil {
			logger.Error(fmt.Sprintf("amber: error fetching sites - %v", err))
			continue
		}

		if len(sites) != 1 {
			logger.Error(fmt.Sprintf("amber: found %d sites, need 1", len(sites)))
			continue
		}
		site := sites[0]

		logger.Debug(fmt.Sprintf("amber: found site %s, about to start polling", site.Id))

		// we typically don't expect this to return
		startPolling(bus, logger, state, client, site)
	}
}

func startPolling(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, client *amberClient.Client, site amberClient.Site) {
	generalCentsPerKwhSensor := entities.NewSensorGauge(bus, "amber.general.cents_per_kwh")
	feedinCentsPerKwhSensor := entities.NewSensorGauge(bus, "amber.feedin.cents_per_kwh")
	spotCentsPerKwhSensor := entities.NewSensorGauge(bus, "amber.general.spot_cents_per_kwh")
	renewablesSensor := entities.NewSensorGauge(bus, "amber.general.renewables")
	cheapAtSensor := entities.NewSensorTime(bus, "amber.general.cheap_at")
	cheapPriceSensor := entities.NewSensorGauge(bus, "amber.general.cheap_price")
	minsUntilCheapSensor := entities.NewSensorGauge(bus, "amber.general.mins_until_cheap")

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
			generalCentsPerKwhSensor.Update(generalPrice.PerKwh)
			spotCentsPerKwhSensor.Update(generalPrice.SpotPerKwh)
			renewablesSensor.Update(generalPrice.Renewables)
		}
		if feedInPrice.Type != "" {
			feedinCentsPerKwhSensor.Update(feedInPrice.PerKwh)
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

				cheapAtSensor.Update(price.StartTime)
				cheapPriceSensor.Update(price.PerKwh)

				minsUntilCheapImports := price.StartTime.Sub(time.Now()) / time.Minute
				minsUntilCheapSensor.Update(float64(minsUntilCheapImports))
			}
		}

		if !foundCheapPrice {
			cheapAtSensor.Unset()
			cheapPriceSensor.Unset()
			minsUntilCheapSensor.Unset()
		}
	}
}
