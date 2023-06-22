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

func (f *filter) simplePattern() {
	for i := range f.Terms {
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

type logicalOperator struct {
	Operator string `parser:"@('AND' | 'OR' | 'NOT')!" json:"operator"`
}

type list struct {
	Values []value
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
	Key          string `parser:"(@Ident"         json:"key,omitempty"`
	AttributeKey string `parser:"('.' @Ident)?)!" json:"attribute-key,omitempty"`
	Operator     string `parser:"@(':' | '=' | '!=' | '!=' | '<' | '<=' | '>=' | '>' | '~' | '!~')!" json:"operator,omitempty"`
	ValuesList   *list  `parser:"(@List" json:"values,omitempty"`
	Value        *value `parser:"| @@)!" json:"value,omitempty"`

	LogicalOperator logicalOperator `parser:"@@?" json:"logical-operator,omitempty"`
}

// key : simple-pattern
// key :( simple-pattern … )
func (t *term) simplePattern() {
	if t.Operator == ":" {
		t.Value.simplePattern()
		if t.ValuesList != nil {
			for i := range t.ValuesList.Values {
				t.ValuesList.Values[i].simplePattern()
			}
		}
	}
}

type value struct {
	Literal                      string  `parser:"  @Ident | @QuotedLiteral"       json:"literal,omitempty"`
	FloatingPointNumericConstant float64 `parser:"| @FloatingPointNumericConstant" json:"floating-point-numeric-constant,omitempty"`
	Integer                      int64   `parser:"| @Int"                          json:"integer,omitempty"`
}

func (v *value) simplePattern() {
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
	{Name: "OperatorSymbols", Pattern: `[!~=:<>.]`},
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
	filter.simplePattern()
	return filter, nil
}

func filterProject(project *resourcemanagerpb.Project, filter *filter) bool {
	return true
}

func FilterProjects(projects []*resourcemanagerpb.Project, filterStr string) ([]*resourcemanagerpb.Project, error) {
	filteredProjects := make([]*resourcemanagerpb.Project, 0, len(projects))
	filter, err := parse(filterStr)
	if err != nil {
		return nil, err
	}
	for _, project := range projects {
		if filterProject(project, filter) {
			filteredProjects = append(filteredProjects, project)
		}
	}
	return filteredProjects, nil
}
