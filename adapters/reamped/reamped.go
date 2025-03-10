package reamped

import (
	"fmt"
	"time"

	conf "github.com/yob/home-data/core/config"
	"github.com/yob/home-data/core/entities"
	"github.com/yob/home-data/core/homestate"
	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/pubsub"
)

const (
	peakCentsPerKwh    = 31.99
	offpeakCentsPerKwh = 20.00
	feedInCentsPerKwh  = 3.30
)

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state homestate.StateReader, config *conf.ConfigSection) {
	generalCentsPerKwhSensor := entities.NewSensorGauge(bus, "reamped.general.cents_per_kwh")
	feedinCentsPerKwhSensor := entities.NewSensorGauge(bus, "reamped.feedin.cents_per_kwh")

	for {
		loc, err := time.LoadLocation("Australia/Melbourne")
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to load timezone: %v", err))
			return
		}
		hour := time.Now().In(loc).Hour()
		if hour < 15 || hour > 20 {
			logger.Debug(fmt.Sprintf("reamped: setting price to offpeak (hour: %d)", hour))
			generalCentsPerKwhSensor.Update(offpeakCentsPerKwh)
		} else {
			logger.Debug(fmt.Sprintf("reamped: setting price to peak"))
			generalCentsPerKwhSensor.Update(peakCentsPerKwh)
		}
		feedinCentsPerKwhSensor.Update(feedInCentsPerKwh)
		time.Sleep(60 * time.Second)
	}

}
