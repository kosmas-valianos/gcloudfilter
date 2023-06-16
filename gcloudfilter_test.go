package gcloudfilter

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	type args struct {
		filter string
	}
	tests := []struct {
		name    string
		args    args
		want    *Filter
		wantErr bool
	}{
		{
			name: "",
			args: args{
				filter: "labels.color=\"red\" parent.id:123 name:HOWL",
			},
			want: &Filter{
				Terms: []*Term{
					{Key: "labels.color", Value: Value{String: "\"red\""}, Operator: "="},
					{Key: "parent.id", Value: Value{Int: 123}, Operator: ":"},
					{Key: "name", Value: Value{UnquotedString: "HOWL"}, Operator: ":"},
				},
				// LogicalOperators: []*LogicalOperators{
				// 	{Operator: "AND"},
				// },
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.args.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			} else {
				t.Log(got)
			}
		})
	}
}
