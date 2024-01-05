package plugin

import (
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"reflect"
	"testing"
	"time"
)

func Test_uniqueRequests(t *testing.T) {
	var (
		date2020 = time.Date(2020, 0, 0, 0, 0, 0, 0, time.UTC)
		date2021 = time.Date(2021, 0, 0, 0, 0, 0, 0, time.UTC)
		date2022 = time.Date(2022, 0, 0, 0, 0, 0, 0, time.UTC)
		date2023 = time.Date(2023, 0, 0, 0, 0, 0, 0, time.UTC)
	)

	type testCase[M HCloudMetrics] struct {
		name     string
		requests []RequestOpts
		want     []RequestOpts
	}
	// Only testing for ServerMetrics because the actual implementation is irrelevant for this method
	tests := []testCase[hcloud.ServerMetrics]{
		{
			name: "single",
			requests: []RequestOpts{
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU},
					TimeRange:    backend.TimeRange{From: date2020, To: date2021},
					Step:         1,
				},
			}, want: []RequestOpts{
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU},
					TimeRange:    backend.TimeRange{From: date2020, To: date2021},
					Step:         1,
				},
			},
		},
		{
			name: "same type, same range",
			requests: []RequestOpts{
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU},
					TimeRange:    backend.TimeRange{From: date2020, To: date2021},
					Step:         1,
				},
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU},
					TimeRange:    backend.TimeRange{From: date2020, To: date2021},
					Step:         1,
				},
			}, want: []RequestOpts{
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU},
					TimeRange:    backend.TimeRange{From: date2020, To: date2021},
					Step:         1,
				},
			},
		},
		{
			name: "different type, same range",
			requests: []RequestOpts{
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU},
					TimeRange:    backend.TimeRange{From: date2020, To: date2021},
					Step:         1,
				},
				{
					MetricsTypes: []MetricsType{MetricsTypeServerDiskBandwidth},
					TimeRange:    backend.TimeRange{From: date2020, To: date2021},
					Step:         1,
				},
			}, want: []RequestOpts{
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU, MetricsTypeServerDiskBandwidth},
					TimeRange:    backend.TimeRange{From: date2020, To: date2021},
					Step:         1,
				},
			},
		},
		{
			name: "same type, different range",
			requests: []RequestOpts{
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU},
					TimeRange:    backend.TimeRange{From: date2020, To: date2021},
					Step:         1,
				},
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU},
					TimeRange:    backend.TimeRange{From: date2022, To: date2023},
					Step:         1,
				},
			}, want: []RequestOpts{
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU},
					TimeRange:    backend.TimeRange{From: date2020, To: date2021},
					Step:         1,
				},
				{
					MetricsTypes: []MetricsType{MetricsTypeServerCPU},
					TimeRange:    backend.TimeRange{From: date2022, To: date2023},
					Step:         1,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uniqueRequests(tt.requests); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("uniqueRequests() = %v, want %v", got, tt.want)
			}
		})
	}
}
