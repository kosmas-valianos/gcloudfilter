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
		f.Terms[i].reguralExpression()
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
	for _, token := range strings.Split(v[0][1:len(v[0])-1], " ") {
		if (token[0] == '"' && token[len(token)-1] == '"') || (token[0] == '\'' && token[len(token)-1] == '\'') {
			l.Values = append(l.Values, value{Literal: token[1 : len(token)-1]})
		} else if integer, err := strconv.ParseInt(token, 10, 64); err == nil {
			l.Values = append(l.Values, value{Integer: integer})
		} else if float, err := strconv.ParseFloat(token, 64); err == nil {
			l.Values = append(l.Values, value{FloatingPointNumericConstant: float})
		} else {
			return fmt.Errorf("token %v is invalid", token)
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
		if t.Value.Literal != "" {
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
				if t.Value != nil && t.Value.Literal == "*" {
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

func (t *term) evaluate(operand string) (bool, error) {
	values := make([]value, 0, 1)
	if t.Value != nil {
		values = append(values, *t.Value)
	} else if t.ValuesList != nil {
		values = t.ValuesList.Values
	}

	var result bool
	var err error
	for _, value := range values {
		switch t.Operator {
		case ":", "~":
			result, err = regexp.MatchString(value.String(), operand)
		case "=":
			result = strings.EqualFold(value.String(), operand)
		case "!=":
			result = !strings.EqualFold(value.String(), operand)
		case "<":
			result = value.String() < operand
		case "<=":
			result = value.String() <= operand
		case ">=":
			result = value.String() >= operand
		case ">":
			result = value.String() > operand
		case "!~":
			result, err = regexp.MatchString(value.String(), operand)
			result = !result
		}
		if result || err != nil {
			break
		}
	}
	return result, err
}

func (t *term) reguralExpression() {
	if t.Operator == ":" {
		// key : simple-pattern
		// key :( simple-pattern … )
		if t.Value != nil {
			t.Value.reguralExpression()
		}
		if t.ValuesList != nil {
			for i := range t.ValuesList.Values {
				t.ValuesList.Values[i].reguralExpression()
			}
		}
	} else if (t.Operator == "~" || t.Operator == "!~") && t.Value != nil {
		// key ~ value
		// True if key contains a match for the RE (regular expression) pattern value
		// key !~ value
		// True if key does not contain a match for the RE (regular expression) pattern value
		t.Value.reguralExpression()
	}
}

type value struct {
	Literal                      string  `parser:"  @Ident | @QuotedLiteral"       json:"literal,omitempty"`
	FloatingPointNumericConstant float64 `parser:"| @FloatingPointNumericConstant" json:"floating-point-numeric-constant,omitempty"`
	Integer                      int64   `parser:"| @Int"                          json:"integer,omitempty"`
}

func (v value) String() string {
	if v.Literal != "" {
		return v.Literal
	} else if v.FloatingPointNumericConstant != 0 {
		return fmt.Sprint(v.FloatingPointNumericConstant)
	} else if v.Integer != 0 {
		return fmt.Sprint(v.Integer)
	}
	return ""
}

func (v *value) reguralExpression() {
	// :* existense. No need for any regexp transformation
	if v.Literal != "" && v.Literal != "*" {
		v.Literal = wildcardToRegexp(v.Literal)
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
		participle.Unquote("QuotedLiteral"),
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
