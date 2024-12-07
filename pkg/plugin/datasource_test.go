package plugin

import (
	"context"
	"reflect"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

func TestQueryData(t *testing.T) {
	ds := Datasource{}

	resp, err := ds.QueryData(
		context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{RefID: "A"},
			},
		},
	)
	if err != nil {
		t.Error(err)
	}

	if len(resp.Responses) != 1 {
		t.Fatal("QueryData must return a response")
	}
}

func Test_getDisplayName(t *testing.T) {
	type args struct {
		legendFormat string
		labels       data.Labels
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Plain Text",
			args: args{
				legendFormat: "Foobar",
				labels:       nil,
			},
			want: "Foobar",
		},
		{
			name: "Auto Format",
			args: args{
				legendFormat: "",
				labels:       data.Labels{LabelName: "webserver", LabelSeriesDisplayName: "Requests"},
			},
			want: "Requests webserver",
		},
		{
			name: "Missing Label",
			args: args{
				legendFormat: "{{ missing }}",
				labels:       data.Labels{},
			},
			want: "",
		},
		{
			name: "Custom Format with weird whitespace",
			args: args{
				legendFormat: "{{ id }} x {{ name}} - {{series_name}} > {{series_display_name   }}",
				labels:       data.Labels{LabelID: "1", LabelName: "Foobar", LabelSeriesName: "requests", LabelSeriesDisplayName: "Requests"},
			},
			want: "1 x Foobar - requests > Requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getDisplayName(tt.args.legendFormat, tt.args.labels); got != tt.want {
				t.Errorf("getDisplayName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sortFrames(t *testing.T) {
	frame := func(id string, seriesName string) *data.Frame {
		return &data.Frame{Fields: []*data.Field{{Labels: data.Labels{
			LabelID:         id,
			LabelSeriesName: seriesName,
		}}}}
	}

	tests := []struct {
		name     string
		input    []*data.Frame
		expected []*data.Frame
	}{
		{
			name:     "Single Frame",
			input:    []*data.Frame{frame("Foo", "")},
			expected: []*data.Frame{frame("Foo", "")},
		},
		{
			name:     "Two Frames Sorted",
			input:    []*data.Frame{frame("Bar", ""), frame("Foo", "")},
			expected: []*data.Frame{frame("Bar", ""), frame("Foo", "")},
		},
		{
			name:     "Two Frames Unsorted",
			input:    []*data.Frame{frame("Foo", ""), frame("Bar", "")},
			expected: []*data.Frame{frame("Bar", ""), frame("Foo", "")},
		},
		{
			name:     "Two Frames by Series Name Sorted",
			input:    []*data.Frame{frame("Foo", "A"), frame("Foo", "B")},
			expected: []*data.Frame{frame("Foo", "A"), frame("Foo", "B")},
		},
		{
			name:     "Two Frames by Series Name Unsorted",
			input:    []*data.Frame{frame("Foo", "B"), frame("Foo", "A")},
			expected: []*data.Frame{frame("Foo", "A"), frame("Foo", "B")},
		},
		{
			name:     "Two Frames by Series Name Equal",
			input:    []*data.Frame{frame("Foo", "A"), frame("Foo", "A")},
			expected: []*data.Frame{frame("Foo", "A"), frame("Foo", "A")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortFrames(tt.input)

			if !reflect.DeepEqual(tt.input, tt.expected) {
				t.Errorf("sortFrames() = %v, want: %v", tt.input, tt.expected)
			}
		})
	}
}
