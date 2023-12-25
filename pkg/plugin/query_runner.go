package plugin

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/apricote/grafana-hcloud-datasource/pkg/set"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/sourcegraph/conc/iter"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
)

type HCloudMetrics interface {
	hcloud.ServerMetrics | hcloud.LoadBalancerMetrics
}

type RequestOpts struct {
	MetricsTypes []MetricsType
	TimeRange    backend.TimeRange
	Step         int
}

type APIRequestFn[M HCloudMetrics] func(ctx context.Context, id int64, opts RequestOpts) (*M, error)
type FilterMetricsFn[M HCloudMetrics] func(metrics *M, metricsTypes []MetricsType) *M

// QueryRunner is responsible for getting the Metrics from the Hetzner Cloud API.
//
// The Hetzner Cloud API has endpoints that expose all metrics for a single resource (server/load-balancer). This runs
// counter to the way you would use the metrics in Grafana, where you would like to see a single metrics for
// multiple resources.
//
// The naive solution to this would send one request per resource per incoming query to the API, but this can easily
// exhaust the API rate limit. The QueryRunner instead buffers incoming requests and only sends a single request to the
// API per resource requested during the buffer period. If you show metrics from the same resource in ie. 5 panels, this
// will only send 1 request to the API instead of 5.
//
// The downside is that responses are slower, because we always wait for the buffer period to end before sending the
// requests.
type QueryRunner[M HCloudMetrics] struct {
	mutex sync.Mutex

	bufferPeriod time.Duration
	bufferTimer  *time.Timer

	apiRequestFn    APIRequestFn[M]
	filterMetricsFn FilterMetricsFn[M]

	requests map[int64][]request[M]
}

func NewQueryRunner[M HCloudMetrics](bufferPeriod time.Duration, apiRequestFn APIRequestFn[M], filterMetrics FilterMetricsFn[M]) *QueryRunner[M] {
	q := &QueryRunner[M]{
		bufferPeriod:    bufferPeriod,
		apiRequestFn:    apiRequestFn,
		filterMetricsFn: filterMetrics,
		requests:        make(map[int64][]request[M]),
	}

	return q
}

type request[M HCloudMetrics] struct {
	opts       RequestOpts
	responseCh chan<- response[M]
}

type response[M HCloudMetrics] struct {
	id   int64
	opts RequestOpts

	metrics *M
	err     error
}

// RequestMetrics requests metrics matching the arguments given.
// It will return a slice of metrics for each id in the same order
func (q *QueryRunner[M]) RequestMetrics(ctx context.Context, ids []int64, opts RequestOpts) (map[int64]*M, error) {
	responseCh := make(chan response[M], len(ids))
	req := request[M]{
		opts:       opts,
		responseCh: responseCh,
	}

	q.mutex.Lock()
	for _, id := range ids {
		q.requests[id] = append(q.requests[id], req)
	}
	q.startBuffer()
	q.mutex.Unlock()

	results := make(map[int64]*M, len(ids))

	for len(results) < len(ids) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case resp := <-responseCh:
			if resp.err != nil {
				// TODO: return partial results? cancel outgoing requests?
				return nil, resp.err
			}

			results[resp.id] = resp.metrics
		}
	}

	return results, nil
}

// startBuffer starts the buffer timer if it's not already running. Caller must hold the mutex.
func (q *QueryRunner[M]) startBuffer() {
	if q.bufferTimer == nil {
		q.bufferTimer = time.AfterFunc(q.bufferPeriod, q.sendRequests)
	}
}

// sendRequests sends the minimal amount of requests necessary to satisfy all
// requests that are in q.requests at the start of the method. It then sends
// responses to all requests that match the response, even if the request was
// only added to q.requests while the API requests were in flight. After that,
// it removes all requests that have been answered from q.requests and resets
// the buffer timer.
func (q *QueryRunner[M]) sendRequests() {
	ctx := context.Background()

	q.mutex.Lock()
	defer q.resetBufferTimer()

	// Actual length might be larger, but it is a reasonable starting point
	allRequests := make([]struct {
		id   int64
		opts RequestOpts
	}, 0, len(q.requests))

	for id, requests := range q.requests {
		id := id
		allOpts := make([]RequestOpts, 0, len(requests))
		for _, req := range requests {
			allOpts = append(allOpts, req.opts)
		}

		uniqueOpts := uniqueRequests(allOpts)

		for _, opts := range uniqueOpts {
			allRequests = append(allRequests, struct {
				id   int64
				opts RequestOpts
			}{id: id, opts: opts})
		}
	}

	// We are finished reading from q for now, lets unlock the mutex until we need it again
	q.mutex.Unlock()

	iter.ForEach(allRequests, func(req *struct {
		id   int64
		opts RequestOpts
	}) {
		metrics, err := q.apiRequestFn(ctx, req.id, req.opts)

		q.sendResponse(response[M]{
			id:   req.id,
			opts: req.opts,

			metrics: metrics,
			err:     err,
		})
	})
}

// sendResponse sends a response to all requests that match it
// and removes them from the q.requests buffer.
// Requires locking q.mutex to remove the requests from the buffer.
func (q *QueryRunner[M]) sendResponse(resp response[M]) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	// Send the response to all open requests that match it
	// Remove all requests that have received a response from q.requests
	newRequestsForID := make([]request[M], 0, len(q.requests[resp.id])-1)
	for _, req := range q.requests[resp.id] {
		if resp.opts.matches(req.opts) {
			req.responseCh <- response[M]{
				id:   resp.id,
				opts: req.opts,

				metrics: q.filterMetricsFn(resp.metrics, req.opts.MetricsTypes),
				err:     resp.err,
			}
		} else {
			newRequestsForID = append(newRequestsForID, req)
		}
	}

	if len(newRequestsForID) == 0 {
		delete(q.requests, resp.id)
	} else {
		q.requests[resp.id] = newRequestsForID
	}
}

// resetBufferTimer will reset the buffer timer so new requests can be sent.
// It will also trigger a new buffer period if unanswered requests remain in the [q.requests]
func (q *QueryRunner[M]) resetBufferTimer() {
	q.bufferTimer = nil

	if len(q.requests) > 0 {
		q.startBuffer()
	}
}

// uniqueRequests deduplicates requests by combining requests with the same time range and step. All metrics types are added together
func uniqueRequests(requests []RequestOpts) []RequestOpts {
	type key struct {
		timeRange backend.TimeRange
		step      int
	}

	unique := make(map[key]set.Set[MetricsType])

	for _, req := range requests {
		k := key{
			timeRange: req.TimeRange,
			step:      req.Step,
		}

		if _, ok := unique[k]; !ok {
			unique[k] = set.New[MetricsType]()
		}

		unique[k].Insert(req.MetricsTypes...)
	}

	uniqueSlice := make([]RequestOpts, 0, len(unique))
	for k, v := range unique {
		metricsTypes := v.ToSlice()
		slices.Sort(metricsTypes) // Make testing possible

		uniqueSlice = append(uniqueSlice, RequestOpts{
			MetricsTypes: metricsTypes,
			TimeRange:    k.timeRange,
			Step:         k.step,
		})
	}

	return uniqueSlice
}

// matches returns true if a response to r can fully satisfy other.
func (r RequestOpts) matches(other RequestOpts) bool {
	timeRangeMatches := r.TimeRange.From == other.TimeRange.From && r.TimeRange.To == other.TimeRange.To
	stepMatches := r.Step == other.Step

	typesMatch := true
	for _, metricsType := range other.MetricsTypes {
		if !slices.Contains(r.MetricsTypes, metricsType) {
			typesMatch = false
			break
		}
	}

	return timeRangeMatches && stepMatches && typesMatch
}
