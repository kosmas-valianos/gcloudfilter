package gcloudfilter

import (
	"testing"
)

func TestParse(t *testing.T) {
	type args struct {
		filter string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Complex",
			args: args{
				filter: `labels.color="red" OR parent.id:2.5E+10 parent.id:-56 OR name:HOWL* AND name:'bOWL*'`,
			},
			want: `{"terms":[{"key":"labels","attribute-key":"color","operator":"=","value":{"literal":"red"},"logical-operator":{"operator":"OR"},"term":{"key":"parent","attribute-key":"id","operator":":","value":{"floating-point-numeric-constant":25000000000},"logical-operator":{"operator":""},"term":{"key":"parent","attribute-key":"id","operator":":","value":{"integer":-56},"logical-operator":{"operator":"OR"},"term":{"key":"name","operator":":","value":{"literal":"^HOWL.*$"},"logical-operator":{"operator":"AND"},"term":{"key":"name","operator":":","value":{"literal":"^bOWL.*$"},"logical-operator":{"operator":""}}}}}}]}`,
		},
		{
			name: "Key defined, Key undefined, Values' list",
			args: args{
				filter: `labels.smell:* AND -labels.volume:* labels.size=("small" 'big' 2.5E+10)`,
			},
			want: `{"terms":[{"key":"labels","attribute-key":"smell","operator":":","value":{"literal":"*"},"logical-operator":{"operator":"AND"},"term":{"key":"-labels","attribute-key":"volume","operator":":","value":{"literal":"*"},"logical-operator":{"operator":""},"term":{"key":"labels","attribute-key":"size","operator":"=","values":{"Values":[{"literal":"small"},{"literal":"big"},{"floating-point-numeric-constant":25000000000}]},"logical-operator":{"operator":""}}}}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse(tt.args.filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got.String() != tt.want {
				t.Errorf("Parse() = %v, want %v", got, tt.want)
			} else {
				t.Log(got)
			}
		})
	}
}
