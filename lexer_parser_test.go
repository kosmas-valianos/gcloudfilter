// gcloudfilter
//
// Copyright 2023 Kosmas Valianos
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcloudfilter

import (
	"testing"
)

func TestParse(t *testing.T) {
	type args struct {
		gcpFilter string
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
				gcpFilter: `labels.c-ol_or="red" OR parent.id:2.5E+10 parent.id:-56 OR name:HOWL* AND name:'bOWL*'`,
			},
			want: `{"terms":[{"key":"labels","attribute-key":"c-ol_or","operator":"=","value":{"literal":"red"},"logical-operator":"OR"},{"key":"parent","attribute-key":"id","operator":":","values":{"values":[{"number":25000000000}]}},{"key":"parent","attribute-key":"id","operator":":","values":{"values":[{"number":-56}]},"logical-operator":"OR"},{"key":"name","operator":":","value":{"literal":"^HOWL.*$"},"logical-operator":"AND"},{"key":"name","operator":":","value":{"literal":"^bOWL.*$"}}]}`,
		},
		{
			name: "Key defined, Key undefined, Values' list",
			args: args{
				gcpFilter: `labels.smell:* AND -labels.volume:* labels.size=(small 'big' 2.5E+10) OR labels.cpu:("sm*all" '*big' 2.5E+10)`,
			},
			want: `{"terms":[{"key":"labels","attribute-key":"smell","operator":":","value":{"literal":"*"},"logical-operator":"AND"},{"negation":true,"key":"labels","attribute-key":"volume","operator":":","value":{"literal":"*"}},{"key":"labels","attribute-key":"size","operator":"=","values":{"values":[{"literal":"small"},{"literal":"big"},{"number":25000000000}]},"logical-operator":"OR"},{"key":"labels","attribute-key":"cpu","operator":":","values":{"values":[{"literal":"^sm.*all$"},{"literal":"^.*big$"},{"number":25000000000}]}}]}`,
		},
		{
			name: "Less common operators",
			args: args{
				gcpFilter: `labels.size >= 50 OR name ~ how* OR name !~ b*ol*`,
			},
			want: `{"terms":[{"key":"labels","attribute-key":"size","operator":"\u003e=","values":{"values":[{"number":50}]},"logical-operator":"OR"},{"key":"name","operator":"~","value":{"literal":"how*"},"logical-operator":"OR"},{"key":"name","operator":"!~","value":{"literal":"b*ol*"}}]}`,
		},
		{
			name: "Negations",
			args: args{
				gcpFilter: `NOT labels.volume:* AND -labels.c-ol_or:*`,
			},
			want: `{"terms":[{"negation":true,"key":"labels","attribute-key":"volume","operator":":","value":{"literal":"*"},"logical-operator":"AND"},{"negation":true,"key":"labels","attribute-key":"c-ol_or","operator":":","value":{"literal":"*"}}]}`,
		},
		{
			name: "Parse error",
			args: args{
				gcpFilter: `NOT labels.volume:* AND labels.c-ol_or/*`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := parser.ParseString("", wrapValuesWithParentheses(quoteStringValues(tt.args.gcpFilter)))
			t.Log(filter)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if filter != nil {
				filter.compileExpression()
				if filter.String() != tt.want {
					t.Errorf("Parse() = %v, want %v", filter, tt.want)
				} else {
					t.Log(filter)
				}
			}
		})
	}
}
