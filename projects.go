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
	"strings"
	"time"

	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
)

type gcpProject struct {
	project *resourcemanagerpb.Project
}

func (g gcpProject) filterTerm(t term) (bool, error) {
	// Search expressions are case insensitive
	key := strings.ToLower(t.Key)
	switch key {
	case "parent":
		attributeKey := strings.ToLower(t.AttributeKey)
		switch attributeKey {
		// e.g. parent:folders/123
		case "":
			return t.evaluate(g.project.GetParent())
		// e.g. parent.type:organization, parent.type:folder
		case "type":
			parentType := strings.Split(g.project.GetParent(), "/")[0]
			return t.evaluate(parentType)
		// e.g. parent.id:123
		case "id":
			parentParts := strings.Split(g.project.GetParent(), "/")
			if len(parentParts) < 2 {
				return false, fmt.Errorf("invalid project's parent %v", g.project.GetParent())
			}
			return t.evaluate(parentParts[1])
		default:
			return false, fmt.Errorf("unknown attribute key %v", t.AttributeKey)
		}
	case "id", "projectid":
		// e.g. id:appgate-dev
		return t.evaluate(g.project.GetProjectId())
	case "state", "lifecyclestate":
		if t.ValuesList != nil {
			if t.ValuesList.Values[0].Literal != nil {
				// e.g. state:ACTIVE
				return t.evaluate(g.project.GetState().String())
			} else {
				// e.g. state:1
				return t.evaluate(fmt.Sprint(g.project.GetState().Number()))
			}
		} else {
			if t.Value.Literal != nil {
				// e.g. state:ACTIVE
				return t.evaluate(g.project.GetState().String())
			} else {
				// e.g. state:1
				return t.evaluate(fmt.Sprint(g.project.GetState().Number()))
			}
		}
	case "displayname", "name":
		return t.evaluate(g.project.GetDisplayName())
	case "createtime":
		return t.evaluateTimestamp(g.project.GetCreateTime().AsTime().Format(time.RFC3339))
	case "updatetime":
		return t.evaluateTimestamp(g.project.GetUpdateTime().AsTime().Format(time.RFC3339))
	case "deletetime":
		return t.evaluateTimestamp(g.project.GetDeleteTime().AsTime().Format(time.RFC3339))
	case "etag":
		return t.evaluate(g.project.GetEtag())
	case "labels":
		// e.g. labels.color:red, labels.color:*, -labels.color:red
		for labelKey, labelValue := range g.project.GetLabels() {
			if labelKey == t.AttributeKey {
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

// FilterProjects filters the given projects according to the gcpFilter
// Notes:
//  1. The query shall comply with https://cloud.google.com/resource-manager/reference/rest/v3/projects/search
func FilterProjects(projects []*resourcemanagerpb.Project, gcpFilter string) ([]*resourcemanagerpb.Project, error) {
	filteredProjects := make([]*resourcemanagerpb.Project, 0, len(projects))
	for _, project := range projects {
		resource := resource[gcpProject]{
			gcpResource: gcpProject{
				project: project,
			},
			gcpFilter: gcpFilter,
		}
		keepProject, err := resource.filter()
		if err != nil {
			return nil, err
		}
		if keepProject {
			filteredProjects = append(filteredProjects, project)
		}
	}
	return filteredProjects, nil
}
