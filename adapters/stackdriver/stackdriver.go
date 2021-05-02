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

func Process(bus *pubsub.Pubsub, googleProject string, localState *sync.Map, stateMap map[string]string) {
	ch_every_minute := bus.Subscribe("every:minute")
	googleProjectID = googleProject
	for _ = range ch_every_minute {
		processEvent(localState, stateMap)
	}
}

func processEvent(localState *sync.Map, stateMap map[string]string) {

	for stateKey, stackdriverMetricName := range stateMap {
		if value, ok := localState.Load(stateKey); ok {
			value64, err := strconv.ParseFloat(value.(string), 8)
			if err == nil {
				stackSubmitGauge(stackdriverMetricName, value64)
			}
		} else {
			fmt.Printf("*** failed to read %s from state\n", stateKey)
		}
	}
}

func stackSubmitGauge(property string, value float64) {
	metricType := fmt.Sprintf("custom.googleapis.com/%s", property)
	ctx := context.Background()
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		fmt.Printf("ERROR: %v", err)
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
	fmt.Printf("Wrote metric to stackdriver: %+v\n", req)

	err = client.CreateTimeSeries(ctx, req)
	if err != nil {
		fmt.Printf("could not write time series value, %v ", err)
		return
	}
	return
}
