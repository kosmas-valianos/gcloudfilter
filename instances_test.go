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
	"reflect"
	"strings"
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
)

type instancesArray []*computepb.Instance

func (i instancesArray) String() string {
	if len(i) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(128)
	for _, instance := range i {
		sb.WriteString(instance.GetName() + " ")
	}
	return sb.String()[:sb.Len()-1]
}

func toBoolPtr(v bool) *bool {
	return &v
}

func TestFilterInstances(t *testing.T) {
	instances := instancesArray{
		{
			Name:         toStringPtr("purple-gateway"),
			CanIpForward: toBoolPtr(false),
			Scheduling: &computepb.Scheduling{
				OnHostMaintenance: toStringPtr("MIGRATE"),
				AutomaticRestart:  toBoolPtr(true),
			},
			DisplayDevice: &computepb.DisplayDevice{
				EnableDisplay: toBoolPtr(false),
			},
			Zone: toStringPtr("https://www.googleapis.com/compute/v1/projects/appgate-dev/zones/europe-west3-c/instances/purple-gateway"),
			Labels: map[string]string{
				"color": "purple",
				"size":  "big",
			},
			LastStartTimestamp: toStringPtr("2020-08-13T06:51:01.450-08:00"),
		},
		{
			Name:         toStringPtr("blue-gateway"),
			CanIpForward: toBoolPtr(false),
			Scheduling: &computepb.Scheduling{
				OnHostMaintenance: toStringPtr("MIGRATE"),
				AutomaticRestart:  toBoolPtr(true),
			},
			DisplayDevice: &computepb.DisplayDevice{
				EnableDisplay: toBoolPtr(false),
			},
			Zone: toStringPtr("https://www.googleapis.com/compute/v1/projects/appgate-dev/zones/europe-west3-a/instances/purple-gateway"),
			Labels: map[string]string{
				"color": "blue",
				"size":  "small",
			},
			LastStartTimestamp: toStringPtr("2020-08-13T06:51:01.450-07:00"),
		},
	}
	type args struct {
		gcpFilter string
	}
	tests := []struct {
		name          string
		args          args
		wantInstances instancesArray
		wantErr       bool
	}{
		{
			name: "Complex 1",
			args: args{
				gcpFilter: `canIpForward:false AND displayDevice.enableDisplay:false scheduling.onHostMaintenance:MIGRATE zone ne ".*europe-west3-a.*" labels.color:purple lastStartTimestamp="2020-08-13T06:51:01.450-08:00"`,
			},
			wantInstances: instancesArray{
				instances[0],
			},
		},
		{
			name: "Complex 2",
			args: args{
				gcpFilter: `NOT canIpForward:true AND displayDevice.enableDisplay:false scheduling.onHostMaintenance:MIGRATE (labels.color:blue OR labels.color:black) zone ~ .*europe-west3-a.*`,
			},
			wantInstances: instancesArray{
				instances[1],
			},
		},
		{
			name: "Wrong key",
			args: args{
				gcpFilter: `canIPForward:false AND displayDevice.enableDisplay:false`,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotInstances, err := FilterInstances(instances, tt.args.gcpFilter)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterForwardingRules() error: \"%v\". wantErr: %v", err, tt.wantErr)
				return
			}
			gotInstancesArray := instancesArray(gotInstances)
			if !reflect.DeepEqual(gotInstancesArray, tt.wantInstances) {
				t.Errorf("FilterInstances(): \"%v\". want: \"%v\"", gotInstancesArray, tt.wantInstances)
			}
			t.Log(gotInstancesArray)
		})
	}
}
