package gcloudfilter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

type Filter struct {
	Terms []Term `parser:"@@+" json:"terms"`
}

func (f *Filter) String() string {
	b, _ := json.Marshal(f)
	return string(b)
}

type LogicalOperator struct {
	Operator string `parser:"@('AND' | 'OR' | 'NOT')!" json:"operator"`
}

type List struct {
	Values []Value
}

func (l *List) Capture(v []string) error {
	for _, token := range strings.Split(v[0][1:len(v[0])-1], " ") {
		if (token[0] == '"' && token[len(token)-1] == '"') || (token[0] == '\'' && token[len(token)-1] == '\'') {
			l.Values = append(l.Values, Value{Literal: token[1 : len(token)-1]})
		} else if integer, err := strconv.ParseInt(token, 10, 64); err == nil {
			l.Values = append(l.Values, Value{Integer: integer})
		} else if float, err := strconv.ParseFloat(token, 64); err == nil {
			l.Values = append(l.Values, Value{FloatingPointNumericConstant: float})
		} else {
			return fmt.Errorf("token %v is invalid", token)
		}
	}
	return nil
}

type Term struct {
	Key          string `parser:"(@Ident"         json:"key,omitempty"`
	AttributeKey string `parser:"('.' @Ident)?)!" json:"attribute-key,omitempty"`
	Operator     string `parser:"@(':' | '=' | '!=' | '!=' | '<' | '<=' | '>=' | '>' | '~' | '!~')!" json:"operator,omitempty"`
	ValuesList   *List  `parser:"(@List" json:"values,omitempty"`
	Value        *Value `parser:"| @@)!"  json:"value,omitempty"`

	LogicalOperator LogicalOperator `parser:"@@?" json:"logical-operator,omitempty"`
	Term            *Term           `parser:"@@?" json:"term,omitempty"`
}

type Value struct {
	Literal                      string  `parser:"  @Ident | @QuotedLiteral"       json:"literal,omitempty"`
	FloatingPointNumericConstant float64 `parser:"| @FloatingPointNumericConstant" json:"floating-point-numeric-constant,omitempty"`
	Integer                      int64   `parser:"| @Int"                          json:"integer,omitempty"`
}

var basicLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Ident", Pattern: `-?[a-zA-Z]+|\*`},
	{Name: "List", Pattern: `\([^\(^\)]*\)`},
	{Name: "QuotedLiteral", Pattern: `"[^"]*"|'[^']*'`},
	{Name: "FloatingPointNumericConstant", Pattern: `[-+]?(\d+\.\d*|\.\d+)([eE][-+]?\d+)?`},
	{Name: "Int", Pattern: `[-+]?\d+`},
	{Name: "OperatorSymbols", Pattern: `[!~=:<>.]`},
	{Name: "Whitespace", Pattern: `\s+`},
})

func Parse(filterStr string) (*Filter, error) {
	parser := participle.MustBuild[Filter](
		participle.Lexer(basicLexer),
		participle.Elide("Whitespace"),
		participle.Unquote("QuotedLiteral"),
	)
	filter, err := parser.ParseString("", filterStr)
	if err != nil {
		return nil, err
	}
	return filter, nil
}
