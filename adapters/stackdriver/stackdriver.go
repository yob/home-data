package stackdriver

import (
	"context"
	"fmt"
	"sync"
	"strconv"
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

func Process(googleProject string, localState *sync.Map, ch_every_minute <-chan pubsub.KeyValueData) {
	googleProjectID = googleProject
	for _ = range ch_every_minute {
		processEvent(localState)
	}
}

// TODO submit more gauges. Maybe we need a config file or something to list them?
func processEvent(localState *sync.Map) {
	if value, ok := localState.Load("kitchen.daikin.temp_inside_celcius"); ok {
		value64, err := strconv.ParseFloat(value.(string), 8)
		if err == nil {
			stackSubmitGauge("kitchen.daikin.temp_inside_celcius", value64)
		}
	} else {
		fmt.Printf("*** failed to read kitchen.daikin.temp_inside_celcius from state\n")
	}

	if value, ok := localState.Load("kitchen.daikin.temp_outside_celcius"); ok {
		value64, err := strconv.ParseFloat(value.(string), 8)
		if err == nil {
			stackSubmitGauge("kitchen.daikin.temp_outside_celcius", value64)
		}
	}
}

func stackSubmitGauge(property string, value float64) {
	metricType := fmt.Sprintf("custom.googleapis.com/%s", property)
	ctx := context.Background()
	c, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		fmt.Printf("ERROR: %v", err)
		return
	}
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

	err = c.CreateTimeSeries(ctx, req)
	if err != nil {
		fmt.Printf("could not write time series value, %v ", err)
		return
	}
	return
}

