package gcloudfilter

import (
	"reflect"
	"testing"

	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
)

func TestParse(t *testing.T) {
	type args struct {
		filterStr string
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
				filterStr: `labels.color="red" OR parent.id:2.5E+10 parent.id:-56 OR name:HOWL* AND name:'bOWL*'`,
			},
			want: `{"terms":[{"key":"labels","attribute-key":"color","operator":"=","value":{"literal":"red"},"logical-operator":"OR"},{"key":"parent","attribute-key":"id","operator":":","value":{"number":25000000000}},{"key":"parent","attribute-key":"id","operator":":","value":{"number":-56},"logical-operator":"OR"},{"key":"name","operator":":","value":{"literal":"^HOWL.*$"},"logical-operator":"AND"},{"key":"name","operator":":","value":{"literal":"^bOWL.*$"}}]}`,
		},
		{
			name: "Key defined, Key undefined, Values' list",
			args: args{
				filterStr: `labels.smell:* AND -labels.volume:* labels.size=("small" 'big' 2.5E+10) OR labels.cpu:("sm*all" '*big' 2.5E+10)`,
			},
			want: `{"terms":[{"key":"labels","attribute-key":"smell","operator":":","value":{"literal":"*"},"logical-operator":"AND"},{"negation":true,"key":"labels","attribute-key":"volume","operator":":","value":{"literal":"*"}},{"key":"labels","attribute-key":"size","operator":"=","values":{"values":[{"literal":"small"},{"literal":"big"},{"number":25000000000}]},"logical-operator":"OR"},{"key":"labels","attribute-key":"cpu","operator":":","values":{"values":[{"literal":"^sm.*all$"},{"literal":"^.*big$"},{"number":25000000000}]}}]}`,
		},
		{
			name: "Less common operators",
			args: args{
				filterStr: `labels.size >= 50 OR name ~ how* OR name !~ b*ol*`,
			},
			want: `{"terms":[{"key":"labels","attribute-key":"size","operator":"\u003e=","value":{"number":50},"logical-operator":"OR"},{"key":"name","operator":"~","value":{"literal":"^how.*$"},"logical-operator":"OR"},{"key":"name","operator":"!~","value":{"literal":"^b.*ol.*$"}}]}`,
		},
		{
			name: "Negations",
			args: args{
				filterStr: `NOT labels.volume:* AND -labels.color:*`,
			},
			want: `{"terms":[{"negation":true,"key":"labels","attribute-key":"volume","operator":":","value":{"literal":"*"},"logical-operator":"AND"},{"negation":true,"key":"labels","attribute-key":"color","operator":":","value":{"literal":"*"}}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse(tt.args.filterStr)
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

func TestFilterProjects(t *testing.T) {
	projects := []*resourcemanagerpb.Project{
		{
			Name:        "projects/82699087620",
			Parent:      "organizations/123",
			DisplayName: "Appgate Dev",
			ProjectId:   "appgate-dev",
			Labels: map[string]string{
				"color":  "red",
				"volume": "big",
				"cpu":    "Intel",
				"size":   "100",
			},
		},
		{
			Name:        "projects/76499083636",
			Parent:      "folders/876",
			DisplayName: "Devops Test",
			ProjectId:   "devops-test",
			Labels: map[string]string{
				"volume": "medium",
				"cpu":    "Intel Skylake",
				"size":   "-2.5E+10",
			},
		},
	}

	type args struct {
		filterStr string
	}
	tests := []struct {
		name    string
		args    args
		want    []*resourcemanagerpb.Project
		wantErr bool
	}{
		{
			name: "Complex 1",
			args: args{
				filterStr: `labels.volume:medium OR parent.type=organizations parent.id<=123 labels.color:red name:appgate* AND NOT labels.smell:* labels.volume:*`,
			},
			want: []*resourcemanagerpb.Project{
				projects[0],
			},
		},
		{
			name: "Complex 2",
			args: args{
				filterStr: `parent:folders* labels.volume:("small",'med*') AND labels.size=(-25000000000 "34" -2.4E+10) AND labels.cpu:("Intel Skylake" foo)`,
			},
			want: []*resourcemanagerpb.Project{
				projects[1],
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterProjects(projects, tt.args.filterStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterProjects() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterProjects() = %v, want %v", got, tt.want)
			}
			for _, project := range got {
				t.Log(project.ProjectId)
			}
		})
	}
}
