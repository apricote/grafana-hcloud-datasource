package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apricote/grafana-hcloud-datasource/pkg/logutil"
	"github.com/grafana/grafana-plugin-sdk-go/build"
	"math"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

var logger = log.DefaultLogger

const (
	QueryTypeResourceList = "resource-list"
	QueryTypeMetrics      = "metrics"
)

type Options struct {
	Debug bool `json:"debug"`
}

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// NewDatasource creates a new datasource instance.
func NewDatasource(_ context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	version := "unknown"
	if buildInfo, err := build.GetBuildInfo(); err != nil {
		logger.Warn("get build info failed", "error", err)
	} else {
		version = buildInfo.Version
	}

	clientOpts := []hcloud.ClientOption{
		hcloud.WithToken(settings.DecryptedSecureJSONData["apiToken"]),
		hcloud.WithApplication("apricote-hcloud-datasource", version),
	}

	options := Options{}
	err := json.Unmarshal(settings.JSONData, &options)
	if err != nil {
		return nil, fmt.Errorf("error parsing options: %w", err)
	}

	if options.Debug {
		logger.Info("Debug logging enabled")
		clientOpts = append(clientOpts, hcloud.WithDebugWriter(logutil.NewDebugWriter(logger)))
	}

	client := hcloud.NewClient(
		clientOpts...,
	)

	return &Datasource{
		client: client,
	}, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct {
	client *hcloud.Client
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using [NewDatasource] factory function.
func (d *Datasource) Dispose() {
	// Clean up datasource instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		var res backend.DataResponse

		switch q.QueryType {
		case QueryTypeResourceList:
			res = d.queryResourceList(ctx, q)
		case QueryTypeMetrics:
			res = d.queryMetrics(ctx, q)
		}

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

type queryModel struct{}

func (d *Datasource) queryResourceList(ctx context.Context, query backend.DataQuery) backend.DataResponse {
	var response backend.DataResponse
	return response
}

func (d *Datasource) queryMetrics(ctx context.Context, query backend.DataQuery) backend.DataResponse {
	var response backend.DataResponse

	// Unmarshal the JSON into our queryModel.
	var qm queryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	step := stepSize(query)

	metrics, _, err := d.client.Server.GetMetrics(ctx, &hcloud.Server{ID: 40502748}, hcloud.ServerGetMetricsOpts{
		Types: []hcloud.ServerMetricType{hcloud.ServerMetricCPU, hcloud.ServerMetricDisk, hcloud.ServerMetricNetwork},
		Start: query.TimeRange.From,
		End:   query.TimeRange.To,
		Step:  step,
	})

	// add the frames to the response.
	response.Frames = append(response.Frames, serverMetricsToFrames(metrics, query.RefID)...)

	return response
}

func stepSize(query backend.DataQuery) int {
	step := int(math.Floor(query.Interval.Seconds()))

	if int64(step) > query.MaxDataPoints {
		// If the query results in more data points than Grafana allows, we need to request a larger step size.
		maxInterval := query.TimeRange.Duration().Seconds() / float64(query.MaxDataPoints)
		step = int(math.Floor(maxInterval))
	}

	if step < 1 {
		step = 1
	}

	return step
}

func serverMetricsToFrames(metrics *hcloud.ServerMetrics, refID string) []*data.Frame {
	frames := make([]*data.Frame, 0, len(metrics.TimeSeries))

	// get all keys in map metrics.TimeSeries
	for name, series := range metrics.TimeSeries {
		frame := data.NewFrame(name)
		frame.RefID = refID

		timestamps := make([]time.Time, 0, len(series))
		values := make([]float64, 0, len(series))

		for _, value := range series {
			// convert float64 to time.Time
			timestamps = append(timestamps, time.Unix(int64(value.Timestamp), 0))

			parsedValue, err := strconv.ParseFloat(value.Value, 64)
			if err != nil {
				// TODO
			}
			values = append(values, parsedValue)
		}

		frame.Fields = append(frame.Fields,
			data.NewField("time", nil, timestamps),
			data.NewField("values", nil, values),
		)

		frames = append(frames, frame)
	}

	return frames
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	_, _, err := d.client.Location.List(ctx, hcloud.LocationListOpts{ListOpts: hcloud.ListOpts{PerPage: 1}})
	if err != nil {
		if hcloud.IsError(err, hcloud.ErrorCodeUnauthorized) {
			return &backend.CheckHealthResult{
				Status:  backend.HealthStatusError,
				Message: "Invalid Token",
			}, nil
		}

		return nil, err
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Successfully connected to Hetzner Cloud API",
	}, nil
}
