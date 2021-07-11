package stackdriver

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/yob/home-data/core/logging"
	"github.com/yob/home-data/core/memorystate"
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

type Config struct {
	GoogleProjectID string
	StateMap        map[string]string
}

func Init(bus *pubsub.Pubsub, logger *logging.Logger, state memorystate.StateReader, config Config) {
	subEveryMinute, _ := bus.Subscribe("every:minute")
	defer subEveryMinute.Close()

	googleProjectID = config.GoogleProjectID
	for _ = range subEveryMinute.Ch {
		processEvent(logger, state, config.StateMap)
	}
}

func processEvent(logger *logging.Logger, state memorystate.StateReader, stateMap map[string]string) {
	for stateKey, stackdriverMetricName := range stateMap {
		if value, ok := state.Read(stateKey); ok {
			value64, err := strconv.ParseFloat(value, 8)
			if err == nil {
				stackSubmitGauge(logger, stackdriverMetricName, value64)
			}
		} else {
			logger.Debug(fmt.Sprintf("stackdriver: failed to read %s from state", stateKey))
		}
	}
}

func stackSubmitGauge(logger *logging.Logger, property string, value float64) {
	metricType := fmt.Sprintf("custom.googleapis.com/%s", property)
	ctx := context.Background()
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		logger.Error(fmt.Sprintf("stackdriver (stackSubmitGauge): %v", err))
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
	logger.Debug(fmt.Sprintf("stackdriver: wrote metric %+v", req))

	err = client.CreateTimeSeries(ctx, req)
	if err != nil {
		logger.Error(fmt.Sprintf("stackdriver: could not write time series value, %v ", err))
		return
	}
	return
}
