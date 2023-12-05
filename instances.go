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

type gcpInstance struct {
	instance *computepb.Instance
}

func (g gcpInstance) filterTerm(t term) (bool, error) {
	switch t.Key {
	case "canIpForward":
		return t.evaluate(strconv.FormatBool(g.instance.GetCanIpForward()))
	case "cpuPlatform":
		return t.evaluate(g.instance.GetCpuPlatform())
	case "creationTimestamp":
		return t.evaluate(g.instance.GetCreationTimestamp())
	case "deletionProtection":
		return t.evaluate(strconv.FormatBool(g.instance.GetDeletionProtection()))
	case "description":
		return t.evaluate(g.instance.GetDescription())
	case "displayDevice":
		const displayDeviceKey = "enableDisplay"
		displayDeviceValue := g.instance.GetDisplayDevice().GetEnableDisplay()
		if displayDeviceKey == t.AttributeKey {
			// Existence check
			if t.Value != nil && t.Value.Literal != nil && *t.Value.Literal == "*" {
				return true, nil
			}
			return t.evaluate(strconv.FormatBool(displayDeviceValue))
		}
		return false, fmt.Errorf("unknown displayDevice key %v", t.AttributeKey)
	case "fingerprint":
		return t.evaluate(g.instance.GetFingerprint())
	case "id":
		return t.evaluate(strconv.FormatUint(g.instance.GetId(), 10))
	case "kind":
		return t.evaluate(g.instance.GetKind())
	case "labelFingerprint":
		return t.evaluate(g.instance.GetLabelFingerprint())
	case "labels":
		for labelKey, labelValue := range g.instance.GetLabels() {
			if labelKey == t.AttributeKey {
				// Existence check
				if t.Value != nil && t.Value.Literal != nil && *t.Value.Literal == "*" {
					return true, nil
				}
				return t.evaluate(labelValue)
			}
		}
		return false, nil
	case "lastStartTimestamp":
		return t.evaluate(g.instance.GetLastStartTimestamp())
	case "lastStopTimestamp":
		return t.evaluate(g.instance.GetLastStopTimestamp())
	case "machineType":
		return t.evaluate(g.instance.GetMachineType())
	case "name":
		return t.evaluate(g.instance.GetName())
	case "scheduling":
		var schedulingValue string
		switch t.AttributeKey {
		case "onHostMaintenance":
			schedulingValue = g.instance.GetScheduling().GetOnHostMaintenance()
		case "provisioningModel":
			schedulingValue = g.instance.GetScheduling().GetProvisioningModel()
		case "automaticRestart":
			schedulingValue = strconv.FormatBool(g.instance.GetScheduling().GetAutomaticRestart())
		case "preemptible":
			schedulingValue = strconv.FormatBool(g.instance.GetScheduling().GetPreemptible())
		default:
			return false, fmt.Errorf("unknown scheduling key %v", t.AttributeKey)
		}
		// Existence check
		if t.Value != nil && t.Value.Literal != nil && *t.Value.Literal == "*" {
			return true, nil
		}
		return t.evaluate(schedulingValue)
	case "selfLink":
		return t.evaluate(g.instance.GetSelfLink())
	case "startRestricted":
		return t.evaluate(strconv.FormatBool(g.instance.GetStartRestricted()))
	case "status":
		return t.evaluate(g.instance.GetStatus())
	case "zone":
		return t.evaluate(g.instance.GetZone())
	default:
		return false, fmt.Errorf("unknown key %v", t.Key)
	}
}

// FilterInstances filters the given instances according to the gcpFilter
// Notes:
//  1. The query shall comply with https://cloud.google.com/compute/docs/reference/rest/v1/instances/aggregatedList
func FilterInstances(instances []*computepb.Instance, gcpFilter string) ([]*computepb.Instance, error) {
	filteredInstances := make([]*computepb.Instance, 0, len(instances))
	for _, instance := range instances {
		resource := resource[gcpInstance]{
			gcpResource: gcpInstance{
				instance: instance,
			},
			gcpFilter: gcpFilter,
		}
		keepInstance, err := resource.filter()
		if err != nil {
			return nil, err
		}
		if keepInstance {
			filteredInstances = append(filteredInstances, instance)
		}
	}
	return filteredInstances, nil
}
