// SPDX-License-Identifier: Apache-2.0
package schedule

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ScheduleType identifies the kind of schedule expression.
type ScheduleType int

const (
	ScheduleTypeRate ScheduleType = iota
	ScheduleTypeCron
	ScheduleTypeAt
)

// ParsedSchedule represents a parsed rate or cron expression.
type ParsedSchedule struct {
	Type       ScheduleType
	Expression string

	// Rate fields
	RateValue int
	RateUnit  string

	// Cron fields
	Cron *CronFields

	// At fields
	AtTime time.Time
}

// CronFields holds parsed 6-field AWS cron: minute hour dom month dow year.
type CronFields struct {
	Minute     FieldSpec
	Hour       FieldSpec
	DayOfMonth FieldSpec
	Month      FieldSpec
	DayOfWeek  FieldSpec
	Year       FieldSpec
}

// FieldSpec represents a single cron field with its allowed values.
type FieldSpec struct {
	Values   []int // sorted set of matching values
	Wildcard bool  // true if * or ?
}

var monthNames = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4,
	"MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8,
	"SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var dayNames = map[string]int{
	"SUN": 1, "MON": 2, "TUE": 3, "WED": 4,
	"THU": 5, "FRI": 6, "SAT": 7,
}

// Parse parses an AWS EventBridge schedule expression.
// Supported formats: rate(N unit), cron(min hour dom month dow year).
func Parse(expression string) (*ParsedSchedule, error) {
	expression = strings.TrimSpace(expression)

	switch {
	case strings.HasPrefix(expression, "rate(") && strings.HasSuffix(expression, ")"):
		return parseRate(expression)
	case strings.HasPrefix(expression, "cron(") && strings.HasSuffix(expression, ")"):
		return parseCron(expression)
	case strings.HasPrefix(expression, "at(") && strings.HasSuffix(expression, ")"):
		return parseAt(expression)
	default:
		return nil, fmt.Errorf("invalid schedule expression: must start with rate(, cron(, or at(")
	}
}

func parseRate(expr string) (*ParsedSchedule, error) {
	inner := expr[5 : len(expr)-1]
	parts := strings.Fields(inner)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid rate expression: expected 'rate(value unit)', got '%s'", expr)
	}

	value, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid rate value: %s", parts[0])
	}
	if value < 1 {
		return nil, fmt.Errorf("rate value must be >= 1, got %d", value)
	}

	unit := strings.ToLower(parts[1])
	switch unit {
	case "minute", "minutes":
		if value == 1 && unit != "minute" {
			return nil, fmt.Errorf("rate value 1 requires singular unit 'minute', got '%s'", parts[1])
		}
		if value > 1 && unit != "minutes" {
			return nil, fmt.Errorf("rate value %d requires plural unit 'minutes', got '%s'", value, parts[1])
		}
	case "hour", "hours":
		if value == 1 && unit != "hour" {
			return nil, fmt.Errorf("rate value 1 requires singular unit 'hour', got '%s'", parts[1])
		}
		if value > 1 && unit != "hours" {
			return nil, fmt.Errorf("rate value %d requires plural unit 'hours', got '%s'", value, parts[1])
		}
	case "day", "days":
		if value == 1 && unit != "day" {
			return nil, fmt.Errorf("rate value 1 requires singular unit 'day', got '%s'", parts[1])
		}
		if value > 1 && unit != "days" {
			return nil, fmt.Errorf("rate value %d requires plural unit 'days', got '%s'", value, parts[1])
		}
	default:
		return nil, fmt.Errorf("invalid rate unit: %s (expected minute(s), hour(s), or day(s))", parts[1])
	}

	return &ParsedSchedule{
		Type:       ScheduleTypeRate,
		Expression: expr,
		RateValue:  value,
		RateUnit:   unit,
	}, nil
}

func parseAt(expr string) (*ParsedSchedule, error) {
	inner := strings.TrimSpace(expr[3 : len(expr)-1])
	if inner == "" {
		return nil, fmt.Errorf("invalid at expression: timestamp is required")
	}

	t, err := time.Parse("2006-01-02T15:04:05", inner)
	if err != nil {
		return nil, fmt.Errorf("invalid at expression: expected format 'at(yyyy-mm-ddThh:mm:ss)', got '%s'", expr)
	}

	return &ParsedSchedule{
		Type:       ScheduleTypeAt,
		Expression: expr,
		AtTime:     t.UTC(),
	}, nil
}

// RateDuration returns the time.Duration for a rate schedule.
func (ps *ParsedSchedule) RateDuration() time.Duration {
	switch {
	case strings.HasPrefix(ps.RateUnit, "minute"):
		return time.Duration(ps.RateValue) * time.Minute
	case strings.HasPrefix(ps.RateUnit, "hour"):
		return time.Duration(ps.RateValue) * time.Hour
	case strings.HasPrefix(ps.RateUnit, "day"):
		return time.Duration(ps.RateValue) * 24 * time.Hour
	}
	return 0
}

// NextFireTime returns the next time this schedule should fire after the given time.
// Returns the zero time if the schedule will never fire again.
func (ps *ParsedSchedule) NextFireTime(after time.Time) time.Time {
	switch ps.Type {
	case ScheduleTypeRate:
		return ps.nextRateFireTime(after)
	case ScheduleTypeCron:
		return ps.nextCronFireTime(after)
	case ScheduleTypeAt:
		if ps.AtTime.After(after) {
			return ps.AtTime
		}
		return time.Time{} // already fired, won't fire again
	}
	return time.Time{}
}

func (ps *ParsedSchedule) nextRateFireTime(after time.Time) time.Time {
	dur := ps.RateDuration()
	if dur == 0 {
		return time.Time{}
	}
	// Rate fires at: after + duration
	return after.Add(dur)
}

func (ps *ParsedSchedule) nextCronFireTime(after time.Time) time.Time {
	cf := ps.Cron
	if cf == nil {
		return time.Time{}
	}

	// Start from the next minute after 'after'
	t := after.UTC().Truncate(time.Minute).Add(time.Minute)

	// Search up to 10 years forward to find a match
	limit := after.Add(10 * 365 * 24 * time.Hour)

	for t.Before(limit) {
		// Check year
		if !cf.Year.Wildcard && !contains(cf.Year.Values, t.Year()) {
			// Advance to next year
			t = time.Date(t.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
			continue
		}

		// Check month
		if !cf.Month.Wildcard && !contains(cf.Month.Values, int(t.Month())) {
			// Advance to next month
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			continue
		}

		// Check day (dom and dow interaction)
		domMatch := cf.DayOfMonth.Wildcard || contains(cf.DayOfMonth.Values, t.Day())
		// AWS cron day-of-week: 1=SUN, 7=SAT. Go: 0=Sunday, 6=Saturday.
		goDow := int(t.Weekday()) + 1 // convert to 1=SUN ... 7=SAT
		dowMatch := cf.DayOfWeek.Wildcard || contains(cf.DayOfWeek.Values, goDow)

		if !domMatch || !dowMatch {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, time.UTC)
			continue
		}

		// Check hour
		if !cf.Hour.Wildcard && !contains(cf.Hour.Values, t.Hour()) {
			t = t.Add(time.Hour)
			t = t.Truncate(time.Hour)
			continue
		}

		// Check minute
		if !cf.Minute.Wildcard && !contains(cf.Minute.Values, t.Minute()) {
			t = t.Add(time.Minute)
			continue
		}

		return t
	}

	return time.Time{}
}

func contains(values []int, v int) bool {
	for _, val := range values {
		if val == v {
			return true
		}
	}
	return false
}

func parseCron(expr string) (*ParsedSchedule, error) {
	inner := expr[5 : len(expr)-1]
	fields := strings.Fields(inner)
	if len(fields) != 6 {
		return nil, fmt.Errorf("cron expression must have 6 fields (minute hour dom month dow year), got %d", len(fields))
	}

	// Validate ? constraint: exactly one of dom or dow must be ?
	domIsQ := fields[2] == "?"
	dowIsQ := fields[4] == "?"
	if domIsQ == dowIsQ {
		return nil, fmt.Errorf("exactly one of day-of-month or day-of-week must be '?'")
	}

	minute, err := parseField(fields[0], 0, 59, nil)
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	hour, err := parseField(fields[1], 0, 23, nil)
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	dom, err := parseField(fields[2], 1, 31, nil)
	if err != nil {
		return nil, fmt.Errorf("day-of-month field: %w", err)
	}
	month, err := parseField(fields[3], 1, 12, monthNames)
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	dow, err := parseField(fields[4], 1, 7, dayNames)
	if err != nil {
		return nil, fmt.Errorf("day-of-week field: %w", err)
	}
	year, err := parseField(fields[5], 1970, 2199, nil)
	if err != nil {
		return nil, fmt.Errorf("year field: %w", err)
	}

	return &ParsedSchedule{
		Type:       ScheduleTypeCron,
		Expression: expr,
		Cron: &CronFields{
			Minute:     minute,
			Hour:       hour,
			DayOfMonth: dom,
			Month:      month,
			DayOfWeek:  dow,
			Year:       year,
		},
	}, nil
}

func parseField(field string, min, max int, names map[string]int) (FieldSpec, error) {
	if field == "*" || field == "?" {
		return FieldSpec{Wildcard: true}, nil
	}

	var allValues []int

	// Split by comma for lists
	parts := strings.Split(field, ",")
	for _, part := range parts {
		values, err := parseFieldPart(part, min, max, names)
		if err != nil {
			return FieldSpec{}, err
		}
		allValues = append(allValues, values...)
	}

	// Deduplicate and sort
	seen := make(map[int]bool)
	var unique []int
	for _, v := range allValues {
		if !seen[v] {
			seen[v] = true
			unique = append(unique, v)
		}
	}
	sortInts(unique)

	return FieldSpec{Values: unique}, nil
}

func parseFieldPart(part string, min, max int, names map[string]int) ([]int, error) {
	// Check for step: */N or range/N
	var step int
	if idx := strings.Index(part, "/"); idx >= 0 {
		stepStr := part[idx+1:]
		s, err := strconv.Atoi(stepStr)
		if err != nil || s < 1 {
			return nil, fmt.Errorf("invalid step value: %s", stepStr)
		}
		step = s
		part = part[:idx]
	}

	// Check for range: A-B
	if idx := strings.Index(part, "-"); idx >= 0 {
		startStr := part[:idx]
		endStr := part[idx+1:]
		start, err := resolveValue(startStr, names)
		if err != nil {
			return nil, err
		}
		end, err := resolveValue(endStr, names)
		if err != nil {
			return nil, err
		}
		if start < min || start > max {
			return nil, fmt.Errorf("value %d out of range [%d-%d]", start, min, max)
		}
		if end < min || end > max {
			return nil, fmt.Errorf("value %d out of range [%d-%d]", end, min, max)
		}
		if start > end {
			return nil, fmt.Errorf("range start %d > end %d", start, end)
		}
		return generateRange(start, end, step), nil
	}

	// Wildcard with step: */N
	if part == "*" {
		return generateRange(min, max, step), nil
	}

	// Single value
	v, err := resolveValue(part, names)
	if err != nil {
		return nil, err
	}
	if v < min || v > max {
		return nil, fmt.Errorf("value %d out of range [%d-%d]", v, min, max)
	}
	if step > 0 {
		return generateRange(v, max, step), nil
	}
	return []int{v}, nil
}

func resolveValue(s string, names map[string]int) (int, error) {
	upper := strings.ToUpper(s)
	if names != nil {
		if v, ok := names[upper]; ok {
			return v, nil
		}
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid value: %s", s)
	}
	return v, nil
}

func generateRange(start, end, step int) []int {
	if step < 1 {
		step = 1
	}
	var result []int
	for v := start; v <= end; v += step {
		result = append(result, v)
	}
	return result
}

func sortInts(a []int) {
	// Simple insertion sort — cron fields are tiny
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}
