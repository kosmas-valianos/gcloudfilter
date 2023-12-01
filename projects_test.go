// gcloudfilter
//
// # Copyright 2023 Kosmas Valianos
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gcloudfilter

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type projectsArray []*resourcemanagerpb.Project

func (p projectsArray) String() string {
	if len(p) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(128)
	for _, project := range p {
		sb.WriteString(project.GetProjectId() + " ")
	}
	return sb.String()[:sb.Len()-1]
}

func TestFilterProjects(t *testing.T) {
	projects := projectsArray{
		{
			Name:        "projects/82699087620",
			Parent:      "organizations/448593862441",
			ProjectId:   "appgate-dev",
			State:       1,
			DisplayName: "Appgate Dev",
			CreateTime:  timestamppb.Now(),
			Etag:        `W/"50f1fa462f4ec213"`,
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
			ProjectId:   "devops-test",
			State:       1,
			DisplayName: "Devops Test",
			CreateTime:  timestamppb.Now(),
			Etag:        `W/"ef2024afcf714f51"`,
			Labels: map[string]string{
				"color":  "blue",
				"volume": "medium",
				"cpu":    "Intel Skylake",
				"size":   "-2.5E+10",
			},
		},
	}

	type args struct {
		gcpFilter string
	}
	tests := []struct {
		name         string
		args         args
		wantProjects projectsArray
		wantErr      bool
	}{
		{
			name: "Complex 1",
			args: args{
				gcpFilter: `labels.volume:medium OR ((((true))) id:appgate-dev parent.type=organizations AND parent.id:448593862441) parent.id:"448593862441*" labels.color:red name:appgate* AND NOT labels.smell:* labels.volume:*`,
			},
			wantProjects: projectsArray{
				projects[0],
			},
		},
		{
			name: "Complex 2",
			args: args{
				gcpFilter: `parent:folders* labels.volume:("small",'med*') name ~ "\w+(\s+\w+)*" AND (labels.size=(-25000000000 "34" -2.4E+10) AND labels.cpu:("Intel Skylake" foo))`,
			},
			wantProjects: projectsArray{
				projects[1],
			},
		},
		{
			name: "Timestamp, State",
			args: args{
				gcpFilter: "createTime <= " + fmt.Sprintf("\"%v\"", time.Now().UTC().Format(time.RFC3339)) + " AND state>=1 AND state=ACTIVE",
			},
			wantProjects: projectsArray{
				projects[0],
				projects[1],
			},
		},
		{
			name: "Conjuction having lower precedence than OR - 0",
			args: args{
				gcpFilter: `labels.volume:medium labels.color:red OR labels.color:blue state=1 labels.cpu:* OR -labels.foo:*`,
			},
			wantProjects: projectsArray{
				projects[1],
			},
		},
		{
			name: "Conjuction having lower precedence than OR - 1",
			args: args{
				gcpFilter: `labels.volume:medium OR labels.size:100 labels.color:blue OR labels.color:red state>0`,
			},
			wantProjects: projectsArray{
				projects[0],
				projects[1],
			},
		},
		{
			name: "Parentheses wrapping the whole filter",
			args: args{
				gcpFilter: `(id=("appgate-dev" "foo") AND (-labels.boo:* OR labels.envy:*))`,
			},
			wantProjects: projectsArray{
				projects[0],
			},
		},
		{
			name: "Unbalanced parentheses",
			args: args{
				gcpFilter: `(id=("appgate-dev" "foo") AND ((-labels.boo:* OR labels.envy:*))`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProjects, err := FilterProjects(projects, tt.args.gcpFilter)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterProjects() error: \"%v\". wantErr: %v", err, tt.wantErr)
				return
			}
			gotProjectsArray := projectsArray(gotProjects)
			if !reflect.DeepEqual(gotProjectsArray, tt.wantProjects) {
				t.Errorf("FilterProjects(): \"%v\". want: \"%v\"", gotProjectsArray, tt.wantProjects)
			}
			t.Log(gotProjectsArray)
		})
	}
}
