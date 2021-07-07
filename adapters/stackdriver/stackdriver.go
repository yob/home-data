package stackdriver

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	pubsub "github.com/yob/home-data/pubsub"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

// TODO avoid package level state
var (
	googleProjectID = ""
)

func Init(bus *pubsub.Pubsub, googleProject string, localState *sync.Map, stateMap map[string]string) {
	subEveryMinute, _ := bus.Subscribe("every:minute")
	defer subEveryMinute.Close()

	googleProjectID = googleProject
	for _ = range subEveryMinute.Ch {
		processEvent(bus.PublishChannel(), localState, stateMap)
	}
}

func processEvent(publish chan pubsub.PubsubEvent, localState *sync.Map, stateMap map[string]string) {
	for stateKey, stackdriverMetricName := range stateMap {
		if value, ok := localState.Load(stateKey); ok {
			value64, err := strconv.ParseFloat(value.(string), 8)
			if err == nil {
				stackSubmitGauge(publish, stackdriverMetricName, value64)
			}
		} else {
			debugLog(publish, fmt.Sprintf("stackdriver: failed to read %s from state", stateKey))
		}
	}
}

func stackSubmitGauge(publish chan pubsub.PubsubEvent, property string, value float64) {
	metricType := fmt.Sprintf("custom.googleapis.com/%s", property)
	ctx := context.Background()
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		errorLog(publish, fmt.Sprintf("stackdriver (stackSubmitGauge): %v", err))
		return
	}
	defer client.Close()
	now := &timestamp.Timestamp{
		Seconds: time.Now().Unix(),
	}
	req := &monitoringpb.CreateTimeSeriesRequest{
		Name: "projects/" + googleProjectID,
		TimeSeries: []*monitoringpb.TimeSeries{{
			Metric: &metricpb.Metric{
				Type: metricType,
			},
			Resource: &monitoredrespb.MonitoredResource{
				Type: "global",
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					StartTime: now,
					EndTime:   now,
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DoubleValue{
						DoubleValue: value,
					},
				},
			}},
		}},
	}
	debugLog(publish, fmt.Sprintf("stackdriver: wrote metric %+v", req))

	err = client.CreateTimeSeries(ctx, req)
	if err != nil {
		errorLog(publish, fmt.Sprintf("stackdriver: could not write time series value, %v ", err))
		return
	}
	return
}

func debugLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.NewKeyValueEvent("DEBUG", message),
	}

}

func errorLog(publish chan pubsub.PubsubEvent, message string) {
	publish <- pubsub.PubsubEvent{
		Topic: "log:new",
		Data:  pubsub.NewKeyValueEvent("ERROR", message),
	}

}
