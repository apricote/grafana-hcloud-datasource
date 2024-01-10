package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
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
	MetricsTypeServerCPU              MetricsType = "cpu"
	MetricsTypeServerDiskBandwidth    MetricsType = "disk-bandwidth"
	MetricsTypeServerDiskIOPS         MetricsType = "disk-iops"
	MetricsTypeServerNetworkBandwidth MetricsType = "network-bandwidth"
	MetricsTypeServerNetworkPPS       MetricsType = "network-pps"

	MetricsTypeLoadBalancerOpenConnections      MetricsType = "open-connections"
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

	LegendFormat string `json:"legendFormat"`
}

type Label string

const (
	LabelID                = "id"
	LabelName              = "name"
	LabelSeriesName        = "series_name"
	LabelSeriesDisplayName = "series_display_name"
)

const (
	AutoLegendFormat = "{{ series_display_name }} {{ name }}"

	// DefaultBufferPeriod is the default buffer period for the QueryRunner.
	DefaultBufferPeriod = 200 * time.Millisecond
)

var legendFormatRegexp = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)

var logger = log.DefaultLogger

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler    = (*Datasource)(nil)
	_ backend.CallResourceHandler = (*Datasource)(nil)
	_ backend.CheckHealthHandler  = (*Datasource)(nil)
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
		hcloud.WithInstrumentation(prometheus.DefaultRegisterer),
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

	d.nameCacheServer = NewNameCache[hcloud.Server](client, d.getServerFn, func(server *hcloud.Server) (int64, string) { return server.ID, server.Name })
	d.nameCacheLoadBalancer = NewNameCache[hcloud.LoadBalancer](client, d.getLoadBalancerFn, func(loadBalancer *hcloud.LoadBalancer) (int64, string) { return loadBalancer.ID, loadBalancer.Name })

	return d, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct {
	client *hcloud.Client

	queryRunnerServer       *QueryRunner[hcloud.ServerMetrics]
	queryRunnerLoadBalancer *QueryRunner[hcloud.LoadBalancerMetrics]

	nameCacheServer       *NameCache[hcloud.Server]
	nameCacheLoadBalancer *NameCache[hcloud.LoadBalancer]
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
	ctxLogger := logger.FromContext(ctx)
	var resp backend.DataResponse

	var qm QueryModel
	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourcePlugin, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	resourceIDs, err := d.GetResourceIDs(ctx, qm)
	if err != nil {
		return backend.ErrDataResponseWithSource(backend.StatusBadRequest, backend.ErrorSourceDownstream, fmt.Sprintf("failed to resolve resources: %v", err.Error()))
	}

	step := stepSize(query.TimeRange, query.Interval, query.MaxDataPoints)

	switch qm.ResourceType {
	case ResourceTypeServer:
		metrics, _ := d.queryRunnerServer.RequestMetrics(ctx, resourceIDs, RequestOpts{
			MetricsTypes: []MetricsType{qm.MetricsType},
			TimeRange:    query.TimeRange,
			Step:         step,
		})
		for id, serverMetrics := range metrics {
			name, err := d.nameCacheServer.Get(ctx, id)
			if err != nil {
				ctxLogger.Warn("failed to get server name", "id", id, "error", err)
				name = ""
			}

			resp.Frames = append(resp.Frames, serverMetricsToFrames(id, name, qm.LegendFormat, serverMetrics)...)
		}
	case ResourceTypeLoadBalancer:
		metrics, _ := d.queryRunnerLoadBalancer.RequestMetrics(ctx, resourceIDs, RequestOpts{
			MetricsTypes: []MetricsType{qm.MetricsType},
			TimeRange:    query.TimeRange,
			Step:         step,
		})
		for id, lbMetrics := range metrics {
			name, err := d.nameCacheLoadBalancer.Get(ctx, id)
			if err != nil {
				ctxLogger.Warn("failed to get load balancer name", "id", id, "error", err)
				name = ""
			}

			resp.Frames = append(resp.Frames, loadBalancerMetricsToFrames(id, name, qm.LegendFormat, lbMetrics)...)
		}
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

func serverMetricsToFrames(id int64, serverName string, legendFormat string, metrics *hcloud.ServerMetrics) []*data.Frame {
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
				frame.AppendNotices(data.Notice{
					Severity: data.NoticeSeverityWarning,
					Text:     fmt.Sprintf("Failed to parse value %q at timestamp %f: %v", value.Value, value.Timestamp, err),
				})
			}
			values = append(values, parsedValue)
		}

		labels := data.Labels{
			LabelID:                strconv.FormatInt(id, 10),
			LabelName:              serverName,
			LabelSeriesName:        name,
			LabelSeriesDisplayName: serverSeriesToDisplayName[name],
		}

		valuesField := data.NewField(name, labels, values)
		valuesField.Config = &data.FieldConfig{
			Unit:              serverSeriesToUnit[name],
			DisplayNameFromDS: getDisplayName(legendFormat, labels),
		}

		frame.Fields = append(frame.Fields,
			data.NewField("time", nil, timestamps),
			valuesField,
		)

		frames = append(frames, frame)
	}

	return frames
}

func loadBalancerMetricsToFrames(id int64, loadBalancerMetrics string, legendFormat string, metrics *hcloud.LoadBalancerMetrics) []*data.Frame {
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
				frame.AppendNotices(data.Notice{
					Severity: data.NoticeSeverityWarning,
					Text:     fmt.Sprintf("Failed to parse value %q at timestamp %f: %v", value.Value, value.Timestamp, err),
				})
			}
			values = append(values, parsedValue)
		}

		labels := data.Labels{
			LabelID:                strconv.FormatInt(id, 10),
			LabelName:              loadBalancerMetrics,
			LabelSeriesName:        name,
			LabelSeriesDisplayName: loadBalancerSeriesToDisplayName[name],
		}

		valuesField := data.NewField(name, labels, values)
		valuesField.Config = &data.FieldConfig{
			Unit:              loadBalancerSeriesToUnit[name],
			DisplayNameFromDS: getDisplayName(legendFormat, labels),
		}

		frame.Fields = append(frame.Fields,
			data.NewField("time", nil, timestamps),
			valuesField,
		)

		frames = append(frames, frame)
	}

	return frames
}

// getDisplayName was inspired by github.com/grafana/grafana/pkg/tsdb/prometheus/querydata.getName()
func getDisplayName(legendFormat string, labels data.Labels) string {
	if legendFormat == "" {
		legendFormat = AutoLegendFormat
	}

	return legendFormatRegexp.ReplaceAllStringFunc(legendFormat, func(in string) string {
		labelName := strings.Replace(in, "{{", "", 1)
		labelName = strings.Replace(labelName, "}}", "", 1)
		labelName = strings.TrimSpace(labelName)
		if val, exists := labels[labelName]; exists {
			return val
		}
		return ""
	})
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

// CallResource handles additional API calls. These are used to fill the resource dropdowns in the query editor.
func (d *Datasource) CallResource(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	ctxLogger := logger.FromContext(ctx)

	var returnData any
	var err error

	if req.Method != http.MethodGet {
		ctxLogger.Warn("unsupported method", "method", req.Method, "path", req.Path)
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusMethodNotAllowed,
		})
	}

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

	d.nameCacheServer.Insert(servers...)

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

	d.nameCacheLoadBalancer.Insert(loadBalancers...)

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

func (d *Datasource) getServerFn(ctx context.Context, id int64) (*hcloud.Server, error) {
	srv, _, err := d.client.Server.GetByID(ctx, id)
	return srv, err
}

func (d *Datasource) getLoadBalancerFn(ctx context.Context, id int64) (*hcloud.LoadBalancer, error) {
	lb, _, err := d.client.LoadBalancer.GetByID(ctx, id)
	return lb, err
}

func (d *Datasource) GetResourceIDs(ctx context.Context, qm QueryModel) ([]int64, error) {
	// If we have an explicit list of IDs use those
	if qm.SelectBy == SelectByID && len(qm.ResourceIDs) > 0 {
		return qm.ResourceIDs, nil
	}

	// If we have a label selector or an empty list of IDs we need to resolve the resources
	listOpts := hcloud.ListOpts{}

	switch qm.SelectBy {
	case SelectByLabel:
		listOpts.LabelSelector = strings.Join(qm.LabelSelectors, ", ")
	case SelectByID:
	// Setting no label selector will return all resources
	default:
		return nil, fmt.Errorf("unknown select by value: %q", qm.SelectBy)
	}

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

		d.nameCacheServer.Insert(servers...)

		var resourceIDs []int64
		for _, server := range servers {
			resourceIDs = append(resourceIDs, server.ID)
		}
		return resourceIDs, nil
	case ResourceTypeLoadBalancer:
		loadBalancers, err := d.client.LoadBalancer.AllWithOpts(ctx, hcloud.LoadBalancerListOpts{
			ListOpts: hcloud.ListOpts{
				LabelSelector: strings.Join(qm.LabelSelectors, ", "),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to resolve resources by label: %w", err)
		}

		d.nameCacheLoadBalancer.Insert(loadBalancers...)

		var resourceIDs []int64
		for _, loadBalancer := range loadBalancers {
			resourceIDs = append(resourceIDs, loadBalancer.ID)
		}
		return resourceIDs, nil
	default:
		return nil, fmt.Errorf("unknown resource type: %q", qm.ResourceType)
	}
}

var (
	serverMetricsTypeSeries = map[MetricsType][]string{
		MetricsTypeServerCPU:              {"cpu"},
		MetricsTypeServerDiskBandwidth:    {"disk.0.bandwidth.read", "disk.0.bandwidth.write"},
		MetricsTypeServerDiskIOPS:         {"disk.0.iops.read", "disk.0.iops.write"},
		MetricsTypeServerNetworkBandwidth: {"network.0.bandwidth.in", "network.0.bandwidth.out"},
		MetricsTypeServerNetworkPPS:       {"network.0.pps.in", "network.0.pps.out"},
	}

	serverSeriesToDisplayName = map[string]string{
		// cpu
		"cpu": "Usage",

		// disk
		"disk.0.iops.read":       "Read",
		"disk.0.iops.write":      "Write",
		"disk.0.bandwidth.read":  "Read",
		"disk.0.bandwidth.write": "Write",

		//network
		"network.0.pps.in":        "Received",
		"network.0.pps.out":       "Sent",
		"network.0.bandwidth.in":  "Received",
		"network.0.bandwidth.out": "Sent",
	}

	serverSeriesToUnit = map[string]string{
		// cpu
		"cpu": "percent",

		// disk
		"disk.0.iops.read":       "iops",
		"disk.0.iops.write":      "iops",
		"disk.0.bandwidth.read":  "binBps",
		"disk.0.bandwidth.write": "binBps",

		//network
		"network.0.pps.in":        "pps",
		"network.0.pps.out":       "pps",
		"network.0.bandwidth.in":  "binBps",
		"network.0.bandwidth.out": "binBps",
	}

	metricTypeToServerMetricType = map[MetricsType]hcloud.ServerMetricType{
		MetricsTypeServerCPU:              hcloud.ServerMetricCPU,
		MetricsTypeServerDiskBandwidth:    hcloud.ServerMetricDisk,
		MetricsTypeServerDiskIOPS:         hcloud.ServerMetricDisk,
		MetricsTypeServerNetworkBandwidth: hcloud.ServerMetricNetwork,
		MetricsTypeServerNetworkPPS:       hcloud.ServerMetricNetwork,
	}

	loadBalancerMetricsTypeSeries = map[MetricsType][]string{
		MetricsTypeLoadBalancerOpenConnections:      {"open_connections"},
		MetricsTypeLoadBalancerConnectionsPerSecond: {"connections_per_second"},
		MetricsTypeLoadBalancerRequestsPerSecond:    {"requests_per_second"},
		MetricsTypeLoadBalancerBandwidth:            {"bandwidth.in", "bandwidth.out"},
	}

	loadBalancerSeriesToDisplayName = map[string]string{
		// open_connections
		"open_connections": "Open Connections",

		// connections_per_second
		"connections_per_second": "Connections Per Second",

		// requests_per_second
		"requests_per_second": "Requests Per Second",

		// bandwidth
		"bandwidth.in":  "Received",
		"bandwidth.out": "Sent",
	}

	loadBalancerSeriesToUnit = map[string]string{
		// open_connections
		"open_connections": "none",

		// connections_per_second
		"connections_per_second": "ops",

		// requests_per_second
		"requests_per_second": "reqps",

		// bandwidth
		"bandwidth.in":  "binBps",
		"bandwidth.out": "binBps",
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
