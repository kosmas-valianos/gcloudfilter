package gcloudfilter

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2"
)

type Filter struct {
	Terms            []*Term             `  @@+`
	LogicalOperators []*LogicalOperators `| @@*`
}

type LogicalOperators struct {
	Operator string `@("AND" | "OR" | "NOT")`
}

type Term struct {
	Key      string `@Ident (@"." @Ident)*`
	Operator string `@(":" | "=" | "<=")`
	Value    Value  `@@`
}

func (f *Filter) String() string {
	var sb strings.Builder
	sb.Grow(256)
	sb.WriteString("Terms:")
	for _, p := range f.Terms {
		sb.WriteString(fmt.Sprint(p))
	}
	return sb.String()
}

func (p *Term) String() string {
	return fmt.Sprintf("\nKey is %v Value is %v Operator is %v", p.Key, p.Value, p.Operator)
}

type Value struct {
	String         string  `  @String`
	UnquotedString string  `| @Ident`
	Float          float64 `| @Float`
	Int            int     `| @Int`
}

func Parse(filterStr string) (*Filter, error) {
	parser := participle.MustBuild[Filter]()
	filter, err := parser.ParseString("", filterStr)
	if err != nil {
		return nil, err
	}
	return filter, nil
}
