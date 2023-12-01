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
	"fmt"
	"strconv"

	"cloud.google.com/go/compute/apiv1/computepb"
)

type gcpForwardingRule struct {
	forwardingRule *computepb.ForwardingRule
}

func (g gcpForwardingRule) filterTerm(t term) (bool, error) {
	switch t.Key {
	case "ipProtocol":
		return t.evaluate(g.forwardingRule.GetIPProtocol())
	case "backendService":
		return t.evaluate(g.forwardingRule.GetBackendService())
	case "creationTimestamp":
		return t.evaluate(g.forwardingRule.GetCreationTimestamp())
	case "description":
		return t.evaluate(g.forwardingRule.GetDescription())
	case "fingerprint":
		return t.evaluate(g.forwardingRule.GetFingerprint())
	case "id":
		return t.evaluate(strconv.FormatUint(g.forwardingRule.GetId(), 10))
	case "ipVersion":
		return t.evaluate(g.forwardingRule.GetIpVersion())
	case "kind":
		return t.evaluate(g.forwardingRule.GetKind())
	case "labelFingerprint":
		return t.evaluate(g.forwardingRule.GetLabelFingerprint())
	case "loadBalancingScheme":
		return t.evaluate(g.forwardingRule.GetLoadBalancingScheme())
	case "name":
		return t.evaluate(g.forwardingRule.GetName())
	case "networkTier":
		return t.evaluate(g.forwardingRule.GetNetworkTier())
	case "portRange":
		return t.evaluate(g.forwardingRule.GetPortRange())
	case "ports":
		if len(g.forwardingRule.GetPorts()) == 1 {
			return t.evaluate(g.forwardingRule.GetPorts()[0])
		}
		return false, fmt.Errorf("expected 1 port, found %v", g.forwardingRule.GetPorts())
	case "region":
		return t.evaluate(g.forwardingRule.GetRegion())
	case "selfLink":
		return t.evaluate(g.forwardingRule.GetSelfLink())
	case "target":
		return t.evaluate(g.forwardingRule.GetTarget())
	case "subnetwork":
		return t.evaluate(g.forwardingRule.GetSubnetwork())
	case "labels":
		// e.g. labels.color:red, labels.color:*, -labels.color:red
		labelKeyFilter := t.AttributeKey
		for labelKey, labelValue := range g.forwardingRule.GetLabels() {
			if labelKey == labelKeyFilter {
				// Existence check
				if t.Value != nil && t.Value.Literal != nil && *t.Value.Literal == "*" {
					return true, nil
				}
				return t.evaluate(labelValue)
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown key %v", t.Key)
	}
}

// FilterForwardingRules filters the given forwarding rules according to the gcpFilter
// Notes:
//  1. The query shall comply with https://cloud.google.com/compute/docs/reference/rest/v1/forwardingRules/aggregatedList
func FilterForwardingRules(forwardingRules []*computepb.ForwardingRule, gcpFilter string) ([]*computepb.ForwardingRule, error) {
	filteredForwardingRules := make([]*computepb.ForwardingRule, 0, len(forwardingRules))
	for _, forwardingRule := range forwardingRules {
		resource := resource[gcpForwardingRule]{
			gcpResource: gcpForwardingRule{
				forwardingRule: forwardingRule,
			},
			gcpFilter: gcpFilter,
		}
		keepForwardingRule, err := resource.filter()
		if err != nil {
			return nil, err
		}
		if keepForwardingRule {
			filteredForwardingRules = append(filteredForwardingRules, forwardingRule)
		}
	}
	return filteredForwardingRules, nil
}
