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

type forwardingRulesArray []*computepb.ForwardingRule

func (f forwardingRulesArray) String() string {
	if len(f) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.Grow(128)
	for _, forwardingRule := range f {
		sb.WriteString(forwardingRule.GetName() + " ")
	}
	return sb.String()[:sb.Len()-1]
}

func toStringPtr(v string) *string {
	return &v
}

func toUint64Ptr(v uint64) *uint64 {
	return &v
}

func TestFilterForwardingRules(t *testing.T) {
	forwardingRules := forwardingRulesArray{
		{
			IPProtocol:          toStringPtr("UDP"),
			BackendService:      toStringPtr("https://www.googleapis.com/compute/v1/projects/appgate-dev/regions/europe-west1/backendServices/lbudp"),
			CreationTimestamp:   toStringPtr("2023-12-01T03:52:49.415-08:00"),
			Description:         toStringPtr("Foo"),
			Fingerprint:         toStringPtr("A71Q0eYvEDM="),
			Id:                  toUint64Ptr(5457014917771327486),
			IpVersion:           toStringPtr("IPV4"),
			Kind:                toStringPtr("compute#forwardingRule"),
			LabelFingerprint:    toStringPtr("42WmSpB8rSM="),
			LoadBalancingScheme: toStringPtr("INTERNAL"),
			Name:                toStringPtr("lbudp-forwarding-rule-2"),
			Region:              toStringPtr("https://www.googleapis.com/compute/v1/projects/appgate-dev/regions/europe-west1"),
			Ports:               []string{"8081"},
			Network:             toStringPtr("https://www.googleapis.com/compute/v1/projects/appgate-dev/global/networks/default"),
			Subnetwork:          toStringPtr("https://www.googleapis.com/compute/v1/projects/appgate-dev/regions/europe-west1/subnetworks/default"),
			NetworkTier:         toStringPtr("PREMIUM"),
			Labels: map[string]string{
				"foo":   "boo",
				"color": "green",
			},
		},
		{
			IPProtocol:          toStringPtr("TCP"),
			Target:              toStringPtr("https://www.googleapis.com/compute/v1/projects/appgate-dev/global/targetHttpProxies/testlbhttp-target-proxy"),
			CreationTimestamp:   toStringPtr("2023-10-24T02:06:40.108-07:00"),
			Description:         toStringPtr("Boo"),
			Fingerprint:         toStringPtr("t3mSldSZEF8="),
			Id:                  toUint64Ptr(1360066178417571791),
			IpVersion:           toStringPtr("IPV4"),
			Kind:                toStringPtr("compute#forwardingRule"),
			LabelFingerprint:    toStringPtr("42WmSpB8rSM="),
			LoadBalancingScheme: toStringPtr("EXTERNAL_MANAGED"),
			Name:                toStringPtr("testlbip"),
			PortRange:           toStringPtr("80-80"),
			NetworkTier:         toStringPtr("PREMIUM"),
			Labels: map[string]string{
				"goo":   "koo",
				"color": "purple",
			},
		},
	}
	type args struct {
		gcpFilter string
	}
	tests := []struct {
		name                string
		args                args
		wantForwardingRules forwardingRulesArray
		wantErr             bool
	}{
		{
			name: "Complex 1",
			args: args{
				gcpFilter: `labels.goo:foo OR ((((true))) id:"1360066178417571791" ipVersion:"IPV4" AND portRange:"80-80") ipProtocol:"TCP*" labels.color:purple name:testlb* AND NOT labels.smell:* labels.goo:*`,
			},
			wantForwardingRules: forwardingRulesArray{
				forwardingRules[1],
			},
		},
		{
			name: "Complex 2",
			args: args{
				gcpFilter: `networkTier=PREMIUM labels.color:("red",'gree*') region ~ ".*europe-west1.*" region eq ".*europe-west1.*" region ne ".*europe-west2.*"`,
			},
			wantForwardingRules: forwardingRulesArray{
				forwardingRules[0],
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotForwardingRules, err := FilterForwardingRules(forwardingRules, tt.args.gcpFilter)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterForwardingRules() error: \"%v\". wantErr: %v", err, tt.wantErr)
				return
			}
			gotForwardingRulesArray := forwardingRulesArray(gotForwardingRules)
			if !reflect.DeepEqual(gotForwardingRulesArray, tt.wantForwardingRules) {
				t.Errorf("FilterForwardingRules(): \"%v\". want: \"%v\"", gotForwardingRulesArray, tt.wantForwardingRules)
			}
			t.Log(gotForwardingRulesArray)
		})
	}
}
