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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

var parser = participle.MustBuild[grammar](
	participle.Lexer(lexer.MustSimple([]lexer.SimpleRule{
		{Name: "Ident", Pattern: `-?[a-zA-Z\*]+|\*`},
		{Name: "List", Pattern: `\([^\(^\)]*\)`},
		{Name: "QuotedLiteral", Pattern: `"[^"]*"|'[^']*'`},
		{Name: "FloatingPointNumericConstant", Pattern: `[-+]?(\d+\.\d*|\.\d+)([eE][-+]?\d+)?`},
		{Name: "Int", Pattern: `[-+]?\d+`},
		{Name: "OperatorSymbols", Pattern: `[!~=:<>.]+`},
		{Name: "Whitespace", Pattern: `\s+`},
	})),
	participle.Elide("Whitespace"),
)

type grammar struct {
	Terms []term `parser:"@@+" json:"terms"`
}

func (g grammar) String() string {
	json, err := json.Marshal(g)
	if err != nil {
		return err.Error()
	}
	return string(json)
}

func (g grammar) compileExpression() {
	for i := range g.Terms {
		if g.Terms[i].SubExpressionResult == nil {
			if g.Terms[i].Key[0] == '-' {
				g.Terms[i].Negation = true
				g.Terms[i].Key = g.Terms[i].Key[1:]
			}
			g.Terms[i].unQuote()
			g.Terms[i].simplePattern()
		}
	}
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

type boolean bool

func (b *boolean) Capture(values []string) error {
	*b = values[0] == "true"
	return nil
}

type term struct {
	Negation            bool     `parser:"((@'NOT'?"                                                   json:"negation,omitempty"`
	Key                 string   `parser:"@Ident"                                                      json:"key,omitempty"`
	AttributeKey        string   `parser:"('.' @Ident)?)!"                                             json:"attribute-key,omitempty"`
	Operator            string   `parser:"@(':' | '=' | '!=' | '<' | '<=' | '>=' | '>' | '~' | '!~')!" json:"operator,omitempty"`
	ValuesList          *list    `parser:"(@List"                                                      json:"values,omitempty"`
	Value               *value   `parser:"| @@)!"                                                      json:"value,omitempty"`
	SubExpressionResult *boolean `parser:"|@('true'|'false'))"                                         json:"subexpression-result,omitempty"`
	LogicalOperator     string   `parser:"@('AND' | 'OR')?"                                            json:"logical-operator,omitempty"`
}

func (t term) evaluateTimestamp(projectTimeStr string) (bool, error) {
	filterValues := make([]value, 0, 1)
	if t.Value != nil {
		filterValues = append(filterValues, *t.Value)
	} else if t.ValuesList != nil {
		filterValues = t.ValuesList.Values
	}

	projectValue := value{Literal: &projectTimeStr}

	var result bool
	var err error
	for _, filterValue := range filterValues {
		if filterValue.Literal == nil {
			return false, errors.New("timestamps can only be compared with RFC3339 time literals")
		}
		// Make sure the value is given in RFC3339 format
		_, err := time.Parse(time.RFC3339, *filterValue.Literal)
		if err != nil {
			return false, err
		}
		result, err = projectValue.compare(t.Operator, filterValue)
		if result || err != nil {
			break
		}
	}
	return result, err
}

func (t term) evaluate(projectValueStr string) (bool, error) {
	filterValues := make([]value, 0, 1)
	if t.Value != nil {
		filterValues = append(filterValues, *t.Value)
	} else if t.ValuesList != nil {
		filterValues = t.ValuesList.Values
	}

	var result bool
	var err error
	for _, filterValue := range filterValues {
		var projectValue value
		if filterValue.Number != nil {
			number, err := strconv.ParseFloat(projectValueStr, 64)
			if err != nil {
				return false, err
			}
			projectValue.Number = &number
		} else {
			projectValue.Literal = &projectValueStr
		}
		result, err = projectValue.compare(t.Operator, filterValue)
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
		sb.WriteString(fmt.Sprintf("\tLiteral: %v\n", *v.Literal))
	}
	if v.Number != nil {
		sb.WriteString(fmt.Sprintf("\tNumber: %v\n", *v.Number))
	}
	return sb.String()
}

func (v value) equal(filterValue value) bool {
	if v.Literal != nil && filterValue.Literal != nil {
		return strings.EqualFold(*v.Literal, *filterValue.Literal)
	} else if v.Number != nil && filterValue.Number != nil {
		return *v.Number == *filterValue.Number
	}
	return false
}

func (v value) lessThan(filterValue value) bool {
	if v.Literal != nil && filterValue.Literal != nil {
		return *v.Literal < *filterValue.Literal
	} else if v.Number != nil && filterValue.Number != nil {
		return *v.Number < *filterValue.Number
	}
	return false
}

func (v value) greaterThan(filterValue value) bool {
	if v.Literal != nil && filterValue.Literal != nil {
		return *v.Literal > *filterValue.Literal
	} else if v.Number != nil && filterValue.Number != nil {
		return *v.Number > *filterValue.Number
	}
	return false
}

func (v value) matchRegExp(filterValue value, simplePattern bool) (bool, error) {
	var pattern string
	if simplePattern {
		pattern = "(?i)"
	}
	if v.Literal != nil && filterValue.Literal != nil {
		return regexp.MatchString(pattern+*filterValue.Literal, *v.Literal)
	} else if v.Number != nil && filterValue.Number != nil {
		filterValueNumber := regexp.QuoteMeta(fmt.Sprint(*filterValue.Number))
		return regexp.MatchString(pattern+filterValueNumber, fmt.Sprint(*v.Number))
	}
	return false, nil
}

func (v value) compare(operator string, filterValue value) (bool, error) {
	switch operator {
	case ":":
		// Case insensitive operator
		return v.matchRegExp(filterValue, true)
	case "=":
		return v.equal(filterValue), nil
	case "!=":
		return !v.equal(filterValue), nil
	case "<":
		return v.lessThan(filterValue), nil
	case "<=":
		result := v.equal(filterValue)
		if !result {
			return v.lessThan(filterValue), nil
		}
		return result, nil
	case ">=":
		result := v.equal(filterValue)
		if !result {
			return v.greaterThan(filterValue), nil
		}
		return result, nil
	case ">":
		return v.greaterThan(filterValue), nil
	case "~":
		return v.matchRegExp(filterValue, false)
	case "!~":
		result, err := v.matchRegExp(filterValue, false)
		if err != nil {
			return false, nil
		}
		return !result, nil
	}
	return false, fmt.Errorf("invalid operator %v", operator)
}

func (v value) simplePattern() {
	// :* existense. No need for any regexp transformation
	if v.Literal != nil && *v.Literal != "*" {
		*v.Literal = wildcardToRegexp(*v.Literal)
	}
}

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

func isOperator(ch byte) bool {
	operators := [...]byte{':', '=', '<', '>', '~', '('}
	for i := range operators {
		if operators[i] == ch {
			return true
		}
	}
	return false
}

func quoteStringValues(gcpFilter string) string {
	var sb strings.Builder
	sb.Grow(len(gcpFilter) + 64)
	var wrap, operator, inQuotes bool
	for i, ch := range gcpFilter {
		if ch == '\'' || ch == '"' {
			inQuotes = !inQuotes
		}

		if inQuotes {
			sb.WriteRune(ch)
			continue
		}

		if isOperator(gcpFilter[i]) {
			operator = true
			sb.WriteRune(ch)
		} else if operator {
			if ch == ' ' && !wrap {
				continue
			}
			if ch == '*' || ch == '\'' || ch == '"' || ch == '-' || ch == '+' || unicode.IsNumber(ch) {
				sb.WriteRune(ch)
			} else {
				sb.WriteRune('"')
				sb.WriteRune(ch)
				wrap = true
			}
			operator = false
		} else if wrap {
			if ch == ' ' {
				sb.WriteRune('"')
				sb.WriteRune(ch)
				wrap = false
			} else if i == len(gcpFilter)-1 {
				sb.WriteRune(ch)
				sb.WriteRune('"')
				wrap = false
			} else {
				sb.WriteRune(ch)
			}
		} else {
			sb.WriteRune(ch)
		}
	}
	return sb.String()
}

// ((labels.color="red" OR parent.id:123.4) OR (name:HOWL AND labels.foo:*)) AND name:'bOWL'
func extractInnermostExpression(gcpFilter string) (string, error) {
	var open, close []int
	var list, quoted bool
	for i, ch := range gcpFilter {
		if ch == '(' {
			if i != 0 && (gcpFilter[i-1] == ':' || gcpFilter[i-1] == '=') {
				// Ignore lists: e.g. labels.volume:("small",'med*')
				list = true
			} else if !quoted {
				// Append only when the parentheses are not inside quotes
				open = append(open, i)
			}
		} else if ch == ')' {
			if list {
				list = false
			} else if !quoted {
				// Append only when the parentheses are not inside quotes
				close = append(close, i)
			}
		} else if ch == '"' || ch == '\'' {
			quoted = !quoted
		}
	}

	if len(open) == 0 && len(close) == 0 {
		return "", nil
	}
	if len(open) != len(close) {
		return "", errors.New("unbalanced parentheses")
	}

	innermostOpen := open[len(open)-1]
	for _, innermostClose := range close {
		if innermostClose > innermostOpen {
			return gcpFilter[innermostOpen+1 : innermostClose], nil
		}
	}
	return "", errors.New("unmatching parentheses")
}

type resourcer interface {
	filterResource(t term) (bool, error)
}

type resource[C resourcer] struct {
	gcpResource C
	gcpFilter   string
}

func (r resource[C]) filterResource() (bool, error) {
	var keepProject bool
	subGCPfilter, err := extractInnermostExpression(r.gcpFilter)
	for ; subGCPfilter != "" && err == nil; subGCPfilter, err = extractInnermostExpression(r.gcpFilter) {
		keepProject, err = r.filterResourceSubExpression(subGCPfilter)
		if err != nil {
			return false, nil
		}
		r.gcpFilter = strings.Replace(r.gcpFilter, "("+subGCPfilter+")", fmt.Sprint(keepProject), 1)
	}
	if err != nil {
		return false, err
	}
	keepProject, err = r.filterResourceSubExpression(r.gcpFilter)
	if err != nil {
		return false, nil
	}
	return keepProject, nil
}

func (r resource[C]) filterResourceSubExpression(gcpFilter string) (bool, error) {
	// Parse the string from subFilterStr into grammar
	grammar, err := parser.ParseString("", quoteStringValues(gcpFilter))
	if err != nil {
		return false, err
	}
	grammar.compileExpression()

	// Evaluate each term according to the given project
	type termResult struct {
		result          bool
		logicalOperator string
	}
	termsResults := make([]termResult, 0, len(grammar.Terms))
	for _, term := range grammar.Terms {
		var result bool
		var err error
		if term.SubExpressionResult != nil {
			result = bool(*term.SubExpressionResult)
		} else {
			result, err = r.gcpResource.filterResource(term)
			if err != nil {
				return false, err
			}
			if term.Negation {
				result = !result
			}
		}
		termsResults = append(termsResults, termResult{result: result, logicalOperator: term.LogicalOperator})
	}

	if len(termsResults) == 1 {
		return termsResults[0].result, nil
	}

	// Do the logical operations left to right. Conjunction has lower precedence than OR
	results := make([]termResult, 0, len(termsResults))
	leftterm := termsResults[0]
	for _, rightTerm := range termsResults[1:] {
		if leftterm.logicalOperator == "OR" {
			leftterm.result = leftterm.result || rightTerm.result
			leftterm.logicalOperator = rightTerm.logicalOperator
		} else if leftterm.logicalOperator == "AND" {
			leftterm.result = leftterm.result && rightTerm.result
			leftterm.logicalOperator = rightTerm.logicalOperator
		} else if leftterm.logicalOperator == "" && (rightTerm.logicalOperator == "AND" || rightTerm.logicalOperator == "") {
			leftterm.result = leftterm.result && rightTerm.result
			leftterm.logicalOperator = rightTerm.logicalOperator
		} else {
			results = append(results, leftterm)
			leftterm = rightTerm
		}
	}
	if len(results) == 0 {
		return leftterm.result, nil
	}

	// Do any remaining conjuctions
	result := results[0].result
	for _, t := range results[1:] {
		result = result && t.result
	}
	return result, nil
}
