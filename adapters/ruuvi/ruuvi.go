package ruuvi

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/tidwall/gjson"
	pubsub "github.com/yob/home-data/pubsub"
)

func HttpHandler(w http.ResponseWriter, r *http.Request, publish chan pubsub.PubsubEvent) (int, error) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return 404, nil
	}

	// TODO: pass this in as config
	var addressMap = map[string]string{
		"cc:64:a6:ed:f6:aa": "study",
		"f2:b0:81:51:8a:e0": "bed1",
		"fb:dd:03:59:e8:26": "bed2",
		"ef:81:7d:23:3c:74": "lounge",
		"c2:69:9e:be:25:aa": "kitchen",
		"fd:54:a9:f0:a8:a5": "outside",
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 400, fmt.Errorf("ERR: %v", err)
	}
	jsonBody := string(body)
	if !gjson.Valid(jsonBody) {
		return 400, fmt.Errorf("invalid JSON")
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
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.temp_celcius", ruuviName), Value: temp.String()},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.humidity", ruuviName), Value: humidity.String()},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.pressure", ruuviName), Value: pressure.String()},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.voltage", ruuviName), Value: voltage.String()},
		}
		publish <- pubsub.PubsubEvent{
			Topic: "state:update",
			Data:  pubsub.KeyValueData{Key: fmt.Sprintf("ruuvi.%s.txpower", ruuviName), Value: txpower.String()},
		}
	}

	fmt.Fprintf(w, "OK")
	return 200, nil
}
