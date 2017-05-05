package filters

import (
	"fmt"
	"regexp"

	"github.com/cloudflare/unsee/models"
)

// FilterT provides methods for interacting with alert filters
type FilterT interface {
	init(name string, matcher *matcherT, rawText string, isValid bool, value string)
	Match(alert *models.Alert, matches int) bool
	GetRawText() string
	GetHits() int
	GetIsValid() bool
}

type alertFilter struct {
	FilterT
	Matched string
	Matcher matcherT
	RawText string
	Value   interface{}
	IsValid bool
	Hits    int
}

func (filter *alertFilter) init(name string, matcher *matcherT, rawText string, isValid bool, value string) {
	filter.Matched = name
	if matcher != nil {
		filter.Matcher = *matcher
	}
	filter.RawText = rawText
	filter.IsValid = isValid
	filter.Value = value
}

func (filter *alertFilter) GetRawText() string {
	return filter.RawText
}

func (filter *alertFilter) GetHits() int {
	return filter.Hits
}

func (filter *alertFilter) GetIsValid() bool {
	return filter.IsValid
}

type newFilterFactory func() FilterT

// NewFilter creates new filter object from filter expression like "key=value"
// expression will be parsed and best filter implementation and value matcher
// will be selected
func NewFilter(expression string) FilterT {
	invalid := alwaysInvalidFilter{}
	invalid.init("", nil, expression, false, expression)

	reExp := fmt.Sprintf("^(?P<matched>(%s))(?P<operator>(%s))(?P<value>(.*))", filterRegex, matcherRegex)
	re := regexp.MustCompile(reExp)
	match := re.FindStringSubmatch(expression)
	result := make(map[string]string)
	for i, name := range re.SubexpNames() {
		if name != "" && i > 0 && i <= len(match) {
			result[name] = match[i]
		}
	}

	matched, _ := result["matched"]
	operator, _ := result["operator"]
	value, _ := result["value"]

	if matched == "" && operator == "" && value == "" {
		// no "filter=" part, just the value, use fuzzy filter
		f := newFuzzyFilter()
		matcher, err := newMatcher(regexpOperator)
		if err != nil {
			f.init("", nil, expression, false, expression)
		} else {
			f.init("", &matcher, expression, true, expression)
		}
		return f
	}

	if value == "" {
		// there's no value, so it's always invalid
		return &invalid
	}

	if operator == "" {
		// no operator, no valid filter here
		return &invalid
	}

	// we have "filter=" part, lookup filter that matches
	for _, fc := range AllFilters {
		f := fc.Factory()
		labelRe := regexp.MustCompile("^(?:" + fc.Label + ")$")
		if !labelRe.MatchString(matched) {
			// filter name doesn't match, keep searching
			continue
		}
		if !stringInSlice(fc.SupportedOperators, operator) {
			return &invalid
		}
		matcher, err := newMatcher(operator)
		if err != nil {
			f.init(matched, nil, expression, false, "")
		} else {
			if value != "" {
				f.init(matched, &matcher, expression, true, value)
				return f
			}
			f.init(matched, &matcher, expression, false, "")
		}
	}

	return &invalid
}
