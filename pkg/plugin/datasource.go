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

const (
	ResourceTypeServer       = "server"
	ResourceTypeLoadBalancer = "load-balancer"
)

type Options struct {
	Debug bool `json:"debug"`
}

type QueryModel struct {
	ResourceType string `json:"resourceType"`
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

	queryData := QueryModel{}
	err := json.Unmarshal(query.JSON, &queryData)
	if err != nil {
		return backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourcePlugin, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	switch queryData.ResourceType {
	case ResourceTypeServer:
		servers, err := d.client.Server.All(ctx)
		if err != nil {
			return backend.ErrDataResponseWithSource(backend.StatusInternal, backend.ErrorSourceDownstream, fmt.Sprintf("error getting servers: %v", err.Error()))
		}

		ids := make([]int64, 0, len(servers))
		names := make([]string, 0, len(servers))
		serverTypes := make([]string, 0, len(servers))
		status := make([]string, 0, len(servers))
		labels := make([]json.RawMessage, 0, len(servers))

		for _, server := range servers {
			ids = append(ids, server.ID)
			names = append(names, server.Name)
			serverTypes = append(serverTypes, server.ServerType.Name)
			status = append(status, string(server.Status))

			labelBytes, err := json.Marshal(server.Labels)
			if err != nil {
				return backend.ErrDataResponseWithSource(backend.StatusInternal, backend.ErrorSourcePlugin, fmt.Sprintf("failed to encode server labels: %v", err.Error()))
			}
			labels = append(labels, labelBytes)
		}

		frame := data.NewFrame("servers")
		frame.Fields = append(frame.Fields,
			data.NewField("id", nil, ids),
			data.NewField("name", nil, names),
			data.NewField("server_type", nil, serverTypes),
			data.NewField("status", nil, status),
			data.NewField("labels", nil, labels),
		)

		response.Frames = append(response.Frames, frame)

	case ResourceTypeLoadBalancer:
		loadBalancers, err := d.client.LoadBalancer.All(ctx)
		if err != nil {
			return backend.ErrDataResponseWithSource(backend.StatusInternal, backend.ErrorSourceDownstream, fmt.Sprintf("error getting load balancers: %v", err.Error()))
		}

		ids := make([]int64, 0, len(loadBalancers))
		names := make([]string, 0, len(loadBalancers))
		loadBalancerTypes := make([]string, 0, len(loadBalancers))
		labels := make([]json.RawMessage, 0, len(loadBalancers))

		for _, lb := range loadBalancers {
			ids = append(ids, lb.ID)
			names = append(names, lb.Name)
			loadBalancerTypes = append(loadBalancerTypes, lb.LoadBalancerType.Name)

			labelBytes, err := json.Marshal(lb.Labels)
			if err != nil {
				return backend.ErrDataResponseWithSource(backend.StatusInternal, backend.ErrorSourcePlugin, fmt.Sprintf("failed to encode load balancer labels: %v", err.Error()))
			}
			labels = append(labels, labelBytes)
		}

		frame := data.NewFrame("load-balancers")
		frame.Fields = append(frame.Fields,
			data.NewField("id", nil, ids),
			data.NewField("name", nil, names),
			data.NewField("load_balancer_type", nil, loadBalancerTypes),
			data.NewField("labels", nil, labels),
		)

		response.Frames = append(response.Frames, frame)
	default:
		return backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourcePlugin, fmt.Sprintf("unknown resource type: %v", queryData.ResourceType))
	}

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

		valuesField := data.NewField("values", nil, values)

		switch name {
		case "cpu":
			valuesField.Config = &data.FieldConfig{
				DisplayName: "CPU Usage",
				Unit:        "percent",
			}
		case "disk.0.iops.read":
			valuesField.Config = &data.FieldConfig{
				DisplayName: "Disk IOPS Read",
				Unit:        "iops",
			}
		case "disk.0.iops.write":
			valuesField.Config = &data.FieldConfig{
				DisplayName: "Disk IOPS Write",
				Unit:        "iops",
			}
		case "disk.0.bandwidth.read":
			valuesField.Config = &data.FieldConfig{
				DisplayName: "Disk Bandwidth Read",
				Unit:        "bytes/sec",
			}
		case "disk.0.bandwidth.write":
			valuesField.Config = &data.FieldConfig{
				DisplayName: "Disk Bandwidth Write",
				Unit:        "bytes/sec",
			}
		case "network.0.pps.in":
			valuesField.Config = &data.FieldConfig{
				DisplayName: "Network PPS Received",
				Unit:        "packets/sec",
			}
		case "network.0.pps.out":
			valuesField.Config = &data.FieldConfig{
				DisplayName: "Network PPS Sent",
				Unit:        "packets/sec",
			}
		case "network.0.bandwidth.in":
			valuesField.Config = &data.FieldConfig{
				DisplayName: "Network Bandwidth Received",
				Unit:        "bytes/sec",
			}
		case "network.0.bandwidth.out":
			valuesField.Config = &data.FieldConfig{
				DisplayName: "Network Bandwidth Sent",
				Unit:        "bytes/sec",
			}
		default:
			// Unknown series, not a problem, we just do not have
			// a good display name and unit
		}

		frame.Fields = append(frame.Fields,
			data.NewField("time", nil, timestamps),
			valuesField,
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
