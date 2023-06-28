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
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

func wildcardToRegexp(pattern string) string {
	components := strings.Split(pattern, "*")
	if len(components) == 1 {
		return "^" + pattern + "$"
	}
	var result strings.Builder
	for i, literal := range components {
		if i > 0 {
			result.WriteString(".*")
		}
		result.WriteString(regexp.QuoteMeta(literal))
	}
	return "^" + result.String() + "$"
}

type filter struct {
	Terms []term `parser:"@@+" json:"terms"`
}

func (f *filter) compileExpression() {
	for i := range f.Terms {
		if f.Terms[i].Key[0] == '-' {
			f.Terms[i].Negation = true
			f.Terms[i].Key = f.Terms[i].Key[1:]
		}
		f.Terms[i].unQuote()
		f.Terms[i].simplePattern()
	}
}

func (f *filter) String() string {
	json, err := json.Marshal(f)
	if err != nil {
		return err.Error()
	}
	return string(json)
}

type list struct {
	Values []value `json:"values,omitempty"`
}

func (l *list) Capture(v []string) error {
	// key :( simple-pattern … )
	// True if key matches any simple-pattern in the (space, tab, newline, comma) separated list
	// key =( value … )
	// True if key is equal to any value in the (space, tab, newline, comma) separated list
	seps := []string{"\t", "\n", " ", ","}
	var tokens []string
	for _, sep := range seps {
		// Ignore separator in case it exists inside single or double quote strings
		// e.g. `"Intel Skylake" 'foo' 54` => 3 tokens
		r := regexp.MustCompile(`(?:"[^"]*"|'[^']*'|[^` + sep + `])+`)
		tokens = r.FindAllString(v[0][1:len(v[0])-1], -1)
		if len(tokens) > 1 {
			break
		}
	}
	for _, token := range tokens {
		if (token[0] == '"' && token[len(token)-1] == '"') || (token[0] == '\'' && token[len(token)-1] == '\'') {
			// Single or double quoted literal
			literal := token[1 : len(token)-1]
			l.Values = append(l.Values, value{Literal: &literal})
		} else if number, err := strconv.ParseFloat(token, 64); err == nil {
			// Number
			l.Values = append(l.Values, value{Number: &number})
		} else {
			// Unquoted literal
			l.Values = append(l.Values, value{Literal: &token})
		}
	}
	return nil
}

type term struct {
	Negation        bool   `parser:"(@'NOT'?"                                                    json:"negation,omitempty"`
	Key             string `parser:"@Ident"                                                      json:"key,omitempty"`
	AttributeKey    string `parser:"('.' @Ident)?)!"                                             json:"attribute-key,omitempty"`
	Operator        string `parser:"@(':' | '=' | '!=' | '<' | '<=' | '>=' | '>' | '~' | '!~')!" json:"operator,omitempty"`
	ValuesList      *list  `parser:"(@List"                                                      json:"values,omitempty"`
	Value           *value `parser:"| @@)!"                                                      json:"value,omitempty"`
	LogicalOperator string `parser:"@('AND' | 'OR')?"                                            json:"logical-operator,omitempty"`
}

func (t *term) filterProject(project *resourcemanagerpb.Project) (bool, error) {
	// Search expressions are case insensitive
	key := strings.ToLower(t.Key)
	switch key {
	case "displayname", "name":
		return t.evaluate(project.DisplayName)
	case "parent":
		attributeKey := strings.ToLower(t.AttributeKey)
		switch attributeKey {
		// e.g. parent:folders/123
		case "":
			return t.evaluate(project.Parent)
		// e.g. parent.type:organization, parent.type:folder
		case "type":
			parentType := strings.Split(project.Parent, "/")[0]
			return t.evaluate(parentType)
		// e.g. parent.id:123
		case "id":
			parentParts := strings.Split(project.Parent, "/")
			if len(parentParts) < 2 {
				return false, fmt.Errorf("invalid project's parent %v", project.Parent)
			}
			return t.evaluate(parentParts[1])
		default:
			return false, fmt.Errorf("unknown attribute key %v", t.AttributeKey)
		}
	case "id", "projectid":
		// e.g. id:appgate-dev
		return t.evaluate(project.ProjectId)
	case "state", "lifecyclestate":
		// e.g. state:ACTIVE
		if t.Value.Literal != nil {
			return t.evaluate(project.State.String())
		}
		// e.g. state:1
		return t.evaluate(fmt.Sprint(project.State.Number()))
	case "labels":
		// e.g. labels.color:red, labels.color:*, -labels.color:red
		labelKeyFilter := t.AttributeKey
		for labelKey, labelValue := range project.Labels {
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

func (t *term) evaluate(projectValueStr string) (bool, error) {
	values := make([]value, 0, 1)
	if t.Value != nil {
		values = append(values, *t.Value)
	} else if t.ValuesList != nil {
		values = t.ValuesList.Values
	}

	var projectValue value
	if number, err := strconv.ParseFloat(projectValueStr, 64); err == nil {
		projectValue.Number = &number
	} else {
		projectValue.Literal = &projectValueStr
	}

	var result bool
	var err error
	for _, value := range values {
		result, err = value.compare(t.Operator, projectValue)
		if result || err != nil {
			break
		}
	}
	return result, err
}

func (t term) unQuote() {
	// Items in t.ValuesList are already unquoted from Capture
	if t.Value != nil && t.Value.Literal != nil {
		literal := *t.Value.Literal
		if (literal[0] == '"' && literal[len(literal)-1] == '"') || (literal[0] == '\'' && literal[len(literal)-1] == '\'') {
			*t.Value.Literal = literal[1 : len(literal)-1]
		}
	}
}

func (t term) simplePattern() {
	if t.Operator == ":" {
		// key : simple-pattern
		// key :( simple-pattern … )
		if t.Value != nil {
			t.Value.simplePattern()
		}
		if t.ValuesList != nil {
			for i := range t.ValuesList.Values {
				t.ValuesList.Values[i].simplePattern()
			}
		}
	}
}

type value struct {
	Literal *string  `parser:"  @Ident | @QuotedLiteral"              json:"literal,omitempty"`
	Number  *float64 `parser:"| @FloatingPointNumericConstant | @Int" json:"number,omitempty"`
}

func (v value) String() string {
	var sb strings.Builder
	sb.WriteString("Value:\n")
	if v.Literal != nil {
		sb.WriteString(fmt.Sprintf("\n\tLiteral: %v", *v.Literal))
	}
	if v.Number != nil {
		sb.WriteString(fmt.Sprintf("\n\tFloating point numeric constant: %v\n", *v.Number))
	}
	return sb.String()
}

func (v value) equal(p value) bool {
	if p.Literal != nil && v.Literal != nil {
		return strings.EqualFold(*v.Literal, *p.Literal)
	} else if p.Number != nil && v.Number != nil {
		return *v.Number == *p.Number
	}
	return false
}

func (v value) lessThan(p value) bool {
	if p.Literal != nil {
		return *v.Literal < *p.Literal
	} else if v.Number != nil {
		return *v.Number < *p.Number
	}
	return false
}

func (v value) GreaterThan(p value) bool {
	if p.Literal != nil {
		return *v.Literal > *p.Literal
	} else if v.Number != nil {
		return *v.Number > *p.Number
	}
	return false
}

func (v value) compare(operator string, p value) (bool, error) {
	switch operator {
	case ":":
		// Case insensitive operator
		return regexp.MatchString("(?i)"+*v.Literal, *p.Literal)
	case "=":
		return v.equal(p), nil
	case "!=":
		return !v.equal(p), nil
	case "<":
		return v.lessThan(p), nil
	case "<=":
		result := v.equal(p)
		if !result {
			return v.lessThan(p), nil
		}
		return result, nil
	case ">=":
		result := v.equal(p)
		if !result {
			return v.GreaterThan(p), nil
		}
		return result, nil
	case ">":
		return v.GreaterThan(p), nil
	case "~":
		return regexp.MatchString(*v.Literal, *p.Literal)
	case "!~":
		result, err := regexp.MatchString(*v.Literal, *p.Literal)
		if err != nil {
			return false, nil
		}
		return !result, nil
	}
	return false, fmt.Errorf("invalid operator %v", operator)
}

func (v *value) simplePattern() {
	// :* existense. No need for any regexp transformation
	if v.Literal != nil && *v.Literal != "*" {
		*v.Literal = wildcardToRegexp(*v.Literal)
	}
}

var basicLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Ident", Pattern: `-?[a-zA-Z\*]+|\*`},
	{Name: "List", Pattern: `\([^\(^\)]*\)`},
	{Name: "QuotedLiteral", Pattern: `"[^"]*"|'[^']*'`},
	{Name: "FloatingPointNumericConstant", Pattern: `[-+]?(\d+\.\d*|\.\d+)([eE][-+]?\d+)?`},
	{Name: "Int", Pattern: `[-+]?\d+`},
	{Name: "OperatorSymbols", Pattern: `[!~=:<>.]+`},
	{Name: "Whitespace", Pattern: `\s+`},
})

func parse(filterStr string) (*filter, error) {
	parser := participle.MustBuild[filter](
		participle.Lexer(basicLexer),
		participle.Elide("Whitespace"),
	)
	filter, err := parser.ParseString("", filterStr)
	if err != nil {
		return nil, err
	}
	filter.compileExpression()
	return filter, nil
}

func filterProject(project *resourcemanagerpb.Project, filter *filter) (bool, error) {
	result, err := filter.Terms[0].filterProject(project)
	if err != nil {
		return false, err
	}
	if filter.Terms[0].Negation {
		result = !result
	}
	logicalOperator := filter.Terms[0].LogicalOperator

	for _, term := range filter.Terms[1:] {
		resultTerm, err := term.filterProject(project)
		if err != nil {
			return false, err
		}
		if term.Negation {
			resultTerm = !resultTerm
		}

		switch logicalOperator {
		// AND, Conjuction. Treat conjuction as an AND
		case "AND", "":
			result = result && resultTerm
		// OR
		case "OR":
			result = result || resultTerm
		}

		logicalOperator = term.LogicalOperator
	}
	return result, nil
}

// FilterProjects filters the given projects according to the filterStr filter
// Notes:
// 1. The grammar and syntax is specified at https://cloud.google.com/sdk/gcloud/reference/topic/filters
// Caveats:
// 1. Parentheses to group expressions like `(labels.color="red" OR parent.id:123.4) OR name:HOWL` are not supported yet
// 2. Conjunction does not have lower precedence than OR
func FilterProjects(projects []*resourcemanagerpb.Project, filterStr string) ([]*resourcemanagerpb.Project, error) {
	filteredProjects := make([]*resourcemanagerpb.Project, 0, len(projects))
	filter, err := parse(filterStr)
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		keepProject, err := filterProject(project, filter)
		if err != nil {
			return nil, err
		}
		if keepProject {
			filteredProjects = append(filteredProjects, project)
		}
	}
	return filteredProjects, nil
}
