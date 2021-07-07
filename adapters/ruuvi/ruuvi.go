package ruuvi

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/tidwall/gjson"
	pubsub "github.com/yob/home-data/pubsub"
)

func Init(bus *pubsub.Pubsub, addressmap map[string]string) {
	publish := bus.PublishChannel()

	publish <- pubsub.PubsubEvent{
		Topic: "http:register-path",
		Data:  pubsub.NewValueEvent("/ruuvi"),
	}

	subRequests, _ := bus.Subscribe("http-request:/ruuvi")
	defer subRequests.Close()

	for event := range subRequests.Ch {
		if event.Type != "http-request" {
			continue
		}
		reqUUID := event.Key

		err := handleRequest(bus, addressmap, event.HttpRequest.Body)
		if err != nil {
			errorLog(publish, fmt.Sprintf("ruuvi: error handling request (%v)", err))

			publish <- pubsub.PubsubEvent{
				Topic: fmt.Sprintf("http-response:%s", reqUUID),
				Data:  pubsub.NewHttpResponseEvent(400, fmt.Sprintf("ERR: %v\n", err), reqUUID),
			}
			continue
		}

		publish <- pubsub.PubsubEvent{
			Topic: fmt.Sprintf("http-response:%s", reqUUID),
			Data:  pubsub.NewHttpResponseEvent(200, "OK\n", reqUUID),
		}
	}
}

func handleRequest(bus *pubsub.Pubsub, addressMap map[string]string, jsonBody string) error {
	publish := bus.PublishChannel()

	if !gjson.Valid(jsonBody) {
		return fmt.Errorf("invalid JSON")
	}

	device_mac := gjson.Get(jsonBody, "device.address")

	if ruuviName, ok := addressMap[device_mac.String()]; ok {
		temp := gjson.Get(jsonBody, "sensors.temperature")
		humidity := gjson.Get(jsonBody, "sensors.humidity")
		pressure := gjson.Get(jsonBody, "sensors.pressure")
		voltage := gjson.Get(jsonBody, "sensors.voltage")
		txpower := gjson.Get(jsonBody, "sensors.txpower")

		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("ruuvi.%s.temp_celcius", ruuviName), temp.String()),
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("ruuvi.%s.humidity", ruuviName), humidity.String()),
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("ruuvi.%s.pressure", ruuviName), pressure.String()),
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("ruuvi.%s.voltage", ruuviName), voltage.String()),
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("ruuvi.%s.txpower", ruuviName), txpower.String()),
		}

		dewpoint, err := calculateDewPoint(temp.Float(), humidity.Float())
		if err == nil {
			publish <- pubsub.PubsubEvent{
				Topic: "state:update",
				Data:  pubsub.NewKeyValueEvent(fmt.Sprintf("ruuvi.%s.dewpoint_celcius", ruuviName), strconv.FormatFloat(dewpoint, 'f', -1, 64)),
			}
		} else {
			errorLog(publish, fmt.Sprintf("ruuvi: error calculating dewpoint - %v", err))
		}
	}

	return nil
}

// From https://github.com/de-wax/go-pkg/blob/a5a606b51a6fa86dc0b561d4d019b3d7fc1e479b/dewpoint/dewpoint.go
// The results match what I get when I plug values into http://www.dpcalc.org/, so maybe they're about right?
func calculateDewPoint(T float64, H float64) (float64, error) {
	// Check if the transferred value for the temperature is within the valid range
	if T < -45 || T > 60 {
		return 0, errors.New("Temperature must be between (-45 - +60°C)")
	}
	// Check if the transferred value for humidity is within the valid range
	if H < 0 || H > 100 {
		return 0, errors.New("Humidity must be between (0 - 100%)")
	}

	// Constants for the Magnus formula
	const a float64 = 17.62
	const b float64 = 243.12

	// Magnus formula
	alpha := math.Log(H/100) + a*T/(b+T)
	return math.Round(((b*alpha)/(a-alpha))*100) / 100, nil
}

func errorLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.NewKeyValueEvent("ERROR", message),
	}

}
