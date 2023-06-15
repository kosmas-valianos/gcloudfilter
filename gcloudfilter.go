package gcloudfilter

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2"
)

type Filter struct {
	Properties []*Term `@@*`
}

func (f *Filter) String() string {
	var sb strings.Builder
	sb.Grow(256)
	sb.WriteString("Properties:")
	for _, p := range f.Properties {
		sb.WriteString(fmt.Sprint(p))
	}
	return sb.String()
}

type Term struct {
	Key      string `@Ident @("." Ident)?`
	Operator string `@(":" | "=" | "<=")`
	Value    Value  `@@`
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

func Parse(filter string) (*Filter, error) {
	parser, err := participle.Build[Filter](
	//participle.Unquote("String"),
	//participle.Union[Value](String{}, Number{}),
	)
	if err != nil {
		return nil, err
	}
	ini, err := parser.ParseString("", filter)
	if err != nil {
		return nil, err
	}
	return ini, nil
}
