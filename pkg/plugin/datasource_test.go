package plugin

import (
	"context"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
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
