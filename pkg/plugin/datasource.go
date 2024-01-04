package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apricote/grafana-hcloud-datasource/pkg/logutil"
	"github.com/grafana/grafana-plugin-sdk-go/build"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sourcegraph/conc/stream"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

const (
	QueryTypeResourceList = "resource-list"
	QueryTypeMetrics      = "metrics"
)

type ResourceType string

const (
	ResourceTypeServer       ResourceType = "server"
	ResourceTypeLoadBalancer ResourceType = "load-balancer"
)

type MetricsType string

const (
	MetricsTypeServerCPU     MetricsType = "cpu"
	MetricsTypeServerDisk    MetricsType = "disk"
	MetricsTypeServerNetwork MetricsType = "network"

	MetricsTypeLoadBalancerOpenConnections      MetricsType = "connections"
	MetricsTypeLoadBalancerConnectionsPerSecond MetricsType = "connections-per-second"
	MetricsTypeLoadBalancerRequestsPerSecond    MetricsType = "requests-per-second"
	MetricsTypeLoadBalancerBandwidth            MetricsType = "bandwidth"
)

type SelectBy string

const (
	SelectByLabel SelectBy = "label"
	SelectByID    SelectBy = "id"
)

type Options struct {
	Debug bool `json:"debug"`
}

type QueryModel struct {
	ResourceType ResourceType `json:"resourceType"`
	MetricsType  MetricsType  `json:"metricsType"`

	SelectBy       SelectBy `json:"selectBy"`
	LabelSelectors []string `json:"labelSelectors"`
	ResourceIDs    []int64  `json:"resourceIds"`
}

const (
	// DefaultBufferPeriod is the default buffer period for the QueryRunner.
	DefaultBufferPeriod = 200 * time.Millisecond
)

var logger = log.DefaultLogger

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CallResourceHandler   = (*Datasource)(nil)
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
		// TODO: Should we update hcloud-go to rely on the registerer interface instead?
		hcloud.WithInstrumentation(prometheus.DefaultRegisterer.(*prometheus.Registry)),
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

	d := &Datasource{
		client: client,
	}

	d.queryRunnerServer = NewQueryRunner[hcloud.ServerMetrics](DefaultBufferPeriod, d.serverAPIRequestFn, filterServerMetrics)
	d.queryRunnerLoadBalancer = NewQueryRunner[hcloud.LoadBalancerMetrics](DefaultBufferPeriod, d.loadBalancerAPIRequestFn, filterLoadBalancerMetrics)

	return d, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct {
	client *hcloud.Client

	queryRunnerServer       *QueryRunner[hcloud.ServerMetrics]
	queryRunnerLoadBalancer *QueryRunner[hcloud.LoadBalancerMetrics]
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
	resp := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	s := stream.New().WithMaxGoroutines(10)
	for _, q := range req.Queries {
		q := q
		s.Go(func() stream.Callback {
			var res backend.DataResponse

			switch q.QueryType {
			case QueryTypeResourceList:
				res = d.queryResourceList(ctx, q)
			case QueryTypeMetrics:
				res = d.queryMetrics(ctx, q)
			}

			// conc makes sure that all callbacks are called in
			// the same goroutine and do not need a mutex
			return func() { resp.Responses[q.RefID] = res }
		})
	}
	s.Wait()

	return resp, nil
}

func (d *Datasource) queryResourceList(ctx context.Context, query backend.DataQuery) backend.DataResponse {
	var resp backend.DataResponse

	queryData := QueryModel{}
	err := json.Unmarshal(query.JSON, &queryData)
	if err != nil {
		return backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourcePlugin, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	switch queryData.ResourceType {
	case ResourceTypeServer:
		servers, err := d.client.Server.AllWithOpts(ctx, hcloud.ServerListOpts{ListOpts: hcloud.ListOpts{LabelSelector: strings.Join(queryData.LabelSelectors, ", ")}})
		if err != nil {
			return backend.ErrDataResponseWithSource(backend.StatusInternal, backend.ErrorSourceDownstream, fmt.Sprintf("error getting servers: %v", err.Error()))
		}

		ids := make([]int64, 0, len(servers))
		vars := make([]string, 0, len(servers))
		names := make([]string, 0, len(servers))
		serverTypes := make([]string, 0, len(servers))
		status := make([]string, 0, len(servers))
		labels := make([]json.RawMessage, 0, len(servers))

		for _, server := range servers {
			ids = append(ids, server.ID)
			vars = append(vars, fmt.Sprintf("%s : %d", server.Name, server.ID))
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
			data.NewField("var", nil, vars),
			data.NewField("name", nil, names),
			data.NewField("server_type", nil, serverTypes),
			data.NewField("status", nil, status),
			data.NewField("labels", nil, labels),
		)

		resp.Frames = append(resp.Frames, frame)

	case ResourceTypeLoadBalancer:
		loadBalancers, err := d.client.LoadBalancer.AllWithOpts(ctx, hcloud.LoadBalancerListOpts{ListOpts: hcloud.ListOpts{LabelSelector: strings.Join(queryData.LabelSelectors, ", ")}})
		if err != nil {
			return backend.ErrDataResponseWithSource(backend.StatusInternal, backend.ErrorSourceDownstream, fmt.Sprintf("error getting load balancers: %v", err.Error()))
		}

		ids := make([]int64, 0, len(loadBalancers))
		vars := make([]string, 0, len(loadBalancers))
		names := make([]string, 0, len(loadBalancers))
		loadBalancerTypes := make([]string, 0, len(loadBalancers))
		labels := make([]json.RawMessage, 0, len(loadBalancers))

		for _, lb := range loadBalancers {
			ids = append(ids, lb.ID)
			vars = append(vars, fmt.Sprintf("%s : %d", lb.Name, lb.ID))
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
			data.NewField("var", nil, vars),
			data.NewField("name", nil, names),
			data.NewField("load_balancer_type", nil, loadBalancerTypes),
			data.NewField("labels", nil, labels),
		)

		resp.Frames = append(resp.Frames, frame)
	default:
		return backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourcePlugin, fmt.Sprintf("unknown resource type: %v", queryData.ResourceType))
	}

	return resp
}

func (d *Datasource) queryMetrics(ctx context.Context, query backend.DataQuery) backend.DataResponse {
	var resp backend.DataResponse

	var qm QueryModel
	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourcePlugin, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	if qm.ResourceType != ResourceTypeServer {
		return backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourcePlugin, fmt.Sprintf("unsupported resouce type: %v", qm.ResourceType))
	}

	resourceIDs, err := d.GetResourceIDs(ctx, qm)
	if err != nil {
		return backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourceDownstream, fmt.Sprintf("failed to resolve resources: %v", err.Error()))
	}

	step := stepSize(query.TimeRange, query.Interval, query.MaxDataPoints)

	metrics, _ := d.queryRunnerServer.RequestMetrics(ctx, resourceIDs, RequestOpts{
		MetricsTypes: []MetricsType{qm.MetricsType},
		TimeRange:    query.TimeRange,
		Step:         step,
	})
	for id, serverMetrics := range metrics {
		resp.Frames = append(resp.Frames, serverMetricsToFrames(id, serverMetrics)...)
	}

	return resp
}

func stepSize(timeRange backend.TimeRange, interval time.Duration, maxDataPoints int64) int {
	step := int(math.Floor(interval.Seconds()))

	if int64(step) > maxDataPoints {
		// If the query results in more data points than Grafana allows, we need to request a larger step size.
		maxInterval := timeRange.Duration().Seconds() / float64(maxDataPoints)
		step = int(math.Floor(maxInterval))
	}

	if step < 1 {
		step = 1
	}

	return step
}

func serverMetricsToFrames(id int64, metrics *hcloud.ServerMetrics) []*data.Frame {
	frames := make([]*data.Frame, 0, len(metrics.TimeSeries))

	// get all keys in map metrics.TimeSeries
	for name, series := range metrics.TimeSeries {
		frame := data.NewFrame("")

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

		valuesField := data.NewField(serverSeriesToDisplayName[name], data.Labels{"id": strconv.FormatInt(id, 10)}, values)
		valuesField.Config = &data.FieldConfig{
			Unit: serverSeriesToUnit[name],
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

func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	ctxLogger := logger.FromContext(ctx)

	var returnData any
	var err error

	switch req.Path {
	case "/servers":
		returnData, err = d.getServers(ctx)
	case "/load-balancers":
		returnData, err = d.getLoadBalancers(ctx)
	}

	if err != nil {
		ctxLogger.Warn("failed to respond to resource call", "path", req.Path, "error", err)
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusInternalServerError,
		})
	}

	body, err := json.Marshal(returnData)
	if err != nil {
		ctxLogger.Warn("failed to encode json body in resource call", "path", req.Path, "error", err)
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusInternalServerError,
		})
	}

	return sender.Send(&backend.CallResourceResponse{
		Status: http.StatusOK,
		Body:   body,
	})
}

type SelectableValue struct {
	Value int64  `json:"value"`
	Label string `json:"label"`
}

func (d *Datasource) getServers(ctx context.Context) ([]SelectableValue, error) {
	servers, err := d.client.Server.All(ctx)
	if err != nil {
		return nil, err
	}

	selectableValues := make([]SelectableValue, 0, len(servers))
	for _, server := range servers {
		selectableValues = append(selectableValues, SelectableValue{
			Value: server.ID,
			Label: server.Name,
		})
	}

	return selectableValues, nil
}

func (d *Datasource) getLoadBalancers(ctx context.Context) ([]SelectableValue, error) {
	loadBalancers, err := d.client.LoadBalancer.All(ctx)
	if err != nil {
		return nil, err
	}

	selectableValues := make([]SelectableValue, 0, len(loadBalancers))
	for _, loadBalancer := range loadBalancers {
		selectableValues = append(selectableValues, SelectableValue{
			Value: loadBalancer.ID,
			Label: loadBalancer.Name,
		})
	}

	return selectableValues, nil
}

func (d *Datasource) serverAPIRequestFn(ctx context.Context, id int64, opts RequestOpts) (*hcloud.ServerMetrics, error) {
	hcloudGoMetricsTypes := make([]hcloud.ServerMetricType, 0, len(opts.MetricsTypes))
	for _, metricsType := range opts.MetricsTypes {
		hcloudGoMetricsTypes = append(hcloudGoMetricsTypes, metricTypeToServerMetricType[metricsType])
	}

	metrics, _, err := d.client.Server.GetMetrics(ctx, &hcloud.Server{ID: id}, hcloud.ServerGetMetricsOpts{
		Types: hcloudGoMetricsTypes,
		Start: opts.TimeRange.From,
		End:   opts.TimeRange.To,
		Step:  opts.Step,
	})

	return metrics, err
}

func (d *Datasource) loadBalancerAPIRequestFn(ctx context.Context, id int64, opts RequestOpts) (*hcloud.LoadBalancerMetrics, error) {
	hcloudGoMetricsTypes := make([]hcloud.LoadBalancerMetricType, 0, len(opts.MetricsTypes))
	for _, metricsType := range opts.MetricsTypes {
		hcloudGoMetricsTypes = append(hcloudGoMetricsTypes, metricTypeToLoadBalancerMetricType[metricsType])
	}

	metrics, _, err := d.client.LoadBalancer.GetMetrics(ctx, &hcloud.LoadBalancer{ID: id}, hcloud.LoadBalancerGetMetricsOpts{
		Types: hcloudGoMetricsTypes,
		Start: opts.TimeRange.From,
		End:   opts.TimeRange.To,
		Step:  opts.Step,
	})

	return metrics, err
}

func (d *Datasource) GetResourceIDs(ctx context.Context, qm QueryModel) ([]int64, error) {
	switch qm.SelectBy {
	case SelectByLabel:

		switch qm.ResourceType {
		case ResourceTypeServer:
			servers, err := d.client.Server.AllWithOpts(ctx, hcloud.ServerListOpts{
				ListOpts: hcloud.ListOpts{
					LabelSelector: strings.Join(qm.LabelSelectors, ", "),
				},
			})
			if err != nil {
				return nil, fmt.Errorf("failed to resolve resources by label: %w", err)
			}

			var resourceIDs []int64
			for _, server := range servers {
				resourceIDs = append(resourceIDs, server.ID)
			}
			return resourceIDs, nil

		case ResourceTypeLoadBalancer:
			return nil, errors.New("not implemented")
		}
	case SelectByID:
		// TODO: Handle empty list
		return qm.ResourceIDs, nil
	default:
		return nil, fmt.Errorf("unknown select by value: %q", qm.SelectBy)
	}

	return nil, fmt.Errorf("unknown error")
}

var (
	serverMetricsTypeSeries = map[MetricsType][]string{
		MetricsTypeServerCPU:     {"cpu"},
		MetricsTypeServerDisk:    {"disk.0.iops.read", "disk.0.iops.write", "disk.0.bandwidth.read", "disk.0.bandwidth.write"},
		MetricsTypeServerNetwork: {"network.0.pps.in", "network.0.pps.out", "network.0.bandwidth.in", "network.0.bandwidth.out"},
	}

	serverSeriesToDisplayName = map[string]string{
		// cpu
		"cpu": "Usage",

		// disk
		"disk.0.iops.read":       "IOPS Read",
		"disk.0.iops.write":      "IOPS Write",
		"disk.0.bandwidth.read":  "Bandwidth Read",
		"disk.0.bandwidth.write": "Bandwidth Write",

		//network
		"network.0.pps.in":        "PPS Received",
		"network.0.pps.out":       "PPS Sent",
		"network.0.bandwidth.in":  "Bandwidth Received",
		"network.0.bandwidth.out": "Bandwidth Sent",
	}

	serverSeriesToUnit = map[string]string{
		// cpu
		"cpu": "percent",

		// disk
		"disk.0.iops.read":       "iops",
		"disk.0.iops.write":      "iops",
		"disk.0.bandwidth.read":  "bytes/sec(IEC)",
		"disk.0.bandwidth.write": "bytes/sec(IEC)",

		//network
		"network.0.pps.in":        "packets/sec",
		"network.0.pps.out":       "packets/sec",
		"network.0.bandwidth.in":  "bytes/sec(IEC)",
		"network.0.bandwidth.out": "bytes/sec(IEC)",
	}

	metricTypeToServerMetricType = map[MetricsType]hcloud.ServerMetricType{
		MetricsTypeServerCPU:     hcloud.ServerMetricCPU,
		MetricsTypeServerDisk:    hcloud.ServerMetricDisk,
		MetricsTypeServerNetwork: hcloud.ServerMetricNetwork,
	}

	loadBalancerMetricsTypeSeries = map[MetricsType][]string{
		MetricsTypeLoadBalancerOpenConnections:      {"open_connections"},
		MetricsTypeLoadBalancerConnectionsPerSecond: {"connections_per_second"},
		MetricsTypeLoadBalancerRequestsPerSecond:    {"requests_per_second"},
		MetricsTypeLoadBalancerBandwidth:            {"bandwidth.in", "bandwidth.out"},
	}

	loadBalancerSeriesToDisplayName = map[string]string{
		// TODO
	}

	loadBalancerSeriesToUnit = map[string]string{
		// TODO
	}

	metricTypeToLoadBalancerMetricType = map[MetricsType]hcloud.LoadBalancerMetricType{
		MetricsTypeLoadBalancerOpenConnections:      hcloud.LoadBalancerMetricOpenConnections,
		MetricsTypeLoadBalancerConnectionsPerSecond: hcloud.LoadBalancerMetricConnectionsPerSecond,
		MetricsTypeLoadBalancerRequestsPerSecond:    hcloud.LoadBalancerMetricRequestsPerSecond,
		MetricsTypeLoadBalancerBandwidth:            hcloud.LoadBalancerMetricBandwidth,
	}
)

func filterServerMetrics(metrics *hcloud.ServerMetrics, metricsTypes []MetricsType) *hcloud.ServerMetrics {
	metricsCopy := *metrics
	metricsCopy.TimeSeries = make(map[string][]hcloud.ServerMetricsValue)

	// For every requested metricsType, copy every series into the copied struct
	for _, metricsType := range metricsTypes {
		for _, series := range serverMetricsTypeSeries[metricsType] {
			metricsCopy.TimeSeries[series] = metrics.TimeSeries[series]
		}
	}

	return &metricsCopy
}

func filterLoadBalancerMetrics(metrics *hcloud.LoadBalancerMetrics, metricsTypes []MetricsType) *hcloud.LoadBalancerMetrics {
	metricsCopy := *metrics
	metricsCopy.TimeSeries = make(map[string][]hcloud.LoadBalancerMetricsValue)

	// For every requested metricsType, copy every series into the copied struct
	for _, metricsType := range metricsTypes {
		for _, series := range loadBalancerMetricsTypeSeries[metricsType] {
			metricsCopy.TimeSeries[series] = metrics.TimeSeries[series]
		}
	}

	return &metricsCopy
}
