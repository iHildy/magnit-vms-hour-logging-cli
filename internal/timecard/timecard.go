package timecard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	SpanTypeLabor = "labor"
	SpanTypeLunch = "lunch"
)

type Span struct {
	Type         string
	Start        string
	End          string
	startMinutes int
	endMinutes   int
}

type SpanSummary struct {
	Type  string `json:"type"`
	Start string `json:"start"`
	End   string `json:"end"`
}

type DaySummary struct {
	WorkedDate string        `json:"worked_date"`
	DidNotWork bool          `json:"did_not_work"`
	Spans      []SpanSummary `json:"spans"`
}

type DayChange struct {
	Date       string     `json:"date"`
	HadExisting bool      `json:"had_existing"`
	Existing   DaySummary `json:"existing"`
	Proposed   DaySummary `json:"proposed"`
}

func ParseDateYYYYMMDD(s string, loc *time.Location) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q, expected YYYY-MM-DD: %w", s, err)
	}
	return t, nil
}

func FormatMDY(t time.Time) string {
	return t.Format("01/02/2006")
}

func WeekStartMonday(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}

func WeekEndSunday(t time.Time) time.Time {
	start := WeekStartMonday(t)
	return start.AddDate(0, 0, 6)
}

func ParseSpanArg(arg string) (Span, error) {
	parts := strings.SplitN(strings.TrimSpace(arg), ":", 2)
	if len(parts) != 2 {
		return Span{}, fmt.Errorf("invalid span %q, expected type:HH:MM-HH:MM", arg)
	}
	spanType := strings.ToLower(strings.TrimSpace(parts[0]))
	if spanType != SpanTypeLabor && spanType != SpanTypeLunch {
		return Span{}, fmt.Errorf("invalid span type %q (allowed: labor, lunch)", spanType)
	}

	timeParts := strings.SplitN(parts[1], "-", 2)
	if len(timeParts) != 2 {
		return Span{}, fmt.Errorf("invalid span %q, expected type:HH:MM-HH:MM", arg)
	}
	start := strings.TrimSpace(timeParts[0])
	end := strings.TrimSpace(timeParts[1])

	startMins, err := parseHHMM(start)
	if err != nil {
		return Span{}, fmt.Errorf("invalid start time in %q: %w", arg, err)
	}
	endMins, err := parseHHMM(end)
	if err != nil {
		return Span{}, fmt.Errorf("invalid end time in %q: %w", arg, err)
	}

	if endMins <= startMins {
		return Span{}, fmt.Errorf("invalid span %q: end must be after start", arg)
	}

	return Span{
		Type:         spanType,
		Start:        start,
		End:          end,
		startMinutes: startMins,
		endMinutes:   endMins,
	}, nil
}

func ValidateSpans(spans []Span) ([]Span, error) {
	if len(spans) == 0 {
		return nil, fmt.Errorf("at least one --span is required")
	}

	sorted := append([]Span(nil), spans...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].startMinutes == sorted[j].startMinutes {
			return sorted[i].endMinutes < sorted[j].endMinutes
		}
		return sorted[i].startMinutes < sorted[j].startMinutes
	})

	for i := 1; i < len(sorted); i++ {
		if sorted[i].startMinutes < sorted[i-1].endMinutes {
			return nil, fmt.Errorf("spans overlap: %s %s-%s and %s %s-%s",
				sorted[i-1].Type, sorted[i-1].Start, sorted[i-1].End,
				sorted[i].Type, sorted[i].Start, sorted[i].End,
			)
		}
	}

	return sorted, nil
}

func PatchDay(metadata map[string]any, targetDate time.Time, spans []Span, markDNW bool) (map[string]any, DayChange, error) {
	copyMetadata, err := deepCopyMap(metadata)
	if err != nil {
		return nil, DayChange{}, fmt.Errorf("copy metadata: %w", err)
	}

	targetMDY := FormatMDY(targetDate)
	weekStart := WeekStartMonday(targetDate)
	weekEnd := WeekEndSunday(targetDate)

	details, ok := anyToSlice(copyMetadata["billingItemDetails"])
	if !ok || len(details) == 0 {
		return nil, DayChange{}, fmt.Errorf("metadata missing billingItemDetails")
	}

	targetIdx := -1
	for i, d := range details {
		detail, ok := anyToMap(d)
		if !ok {
			continue
		}
		if strings.TrimSpace(anyToString(detail["workedDate"])) == targetMDY {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return nil, DayChange{}, fmt.Errorf("date %s not found in current week metadata", targetMDY)
	}

	detail, _ := anyToMap(details[targetIdx])
	existing := extractDaySummary(detail, targetMDY)

	detail["workedDate"] = targetMDY
	detail["didNotWork"] = markDNW

	if markDNW {
		detail["timeEntrySpanDtos"] = nil
	} else {
		detail["timeEntrySpanDtos"] = buildSpanDTOs(targetMDY, spans)
	}

	timeEntry, _ := anyToMap(detail["timeEntry"])
	if timeEntry == nil {
		timeEntry = map[string]any{}
	}
	if _, ok := timeEntry["id"]; !ok {
		timeEntry["id"] = 0
	}
	if _, ok := timeEntry["notes"]; !ok || timeEntry["notes"] == nil {
		timeEntry["notes"] = ""
	}
	timeEntry["daily"] = false
	timeEntry["didNotWork"] = markDNW
	timeEntry["dayOffType"] = "Undefined"
	if markDNW {
		timeEntry["dateWorked"] = nil
		timeEntry["noBreakTaken"] = false
	} else {
		timeEntry["dateWorked"] = targetMDY
		timeEntry["noBreakTaken"] = !containsLunch(spans)
	}
	detail["timeEntry"] = timeEntry

	details[targetIdx] = detail
	copyMetadata["billingItemDetails"] = details

	ensureTopLevel(copyMetadata, weekStart, weekEnd)

	proposed := extractDaySummary(detail, targetMDY)
	change := DayChange{
		Date:       targetMDY,
		HadExisting: existing.DidNotWork || len(existing.Spans) > 0,
		Existing:   existing,
		Proposed:   proposed,
	}

	return copyMetadata, change, nil
}

func FindDaySummary(metadata map[string]any, targetDate time.Time) (DaySummary, error) {
	targetMDY := FormatMDY(targetDate)
	details, ok := anyToSlice(metadata["billingItemDetails"])
	if !ok {
		return DaySummary{}, fmt.Errorf("metadata missing billingItemDetails")
	}
	for _, d := range details {
		detail, ok := anyToMap(d)
		if !ok {
			continue
		}
		if strings.TrimSpace(anyToString(detail["workedDate"])) == targetMDY {
			return extractDaySummary(detail, targetMDY), nil
		}
	}
	return DaySummary{}, fmt.Errorf("date %s not found", targetMDY)
}

func FormatDaySummaryHuman(d DaySummary) string {
	if d.DidNotWork {
		return fmt.Sprintf("%s: did not work", d.WorkedDate)
	}
	if len(d.Spans) == 0 {
		return fmt.Sprintf("%s: no spans", d.WorkedDate)
	}

	parts := make([]string, 0, len(d.Spans))
	for _, s := range d.Spans {
		parts = append(parts, fmt.Sprintf("%s %s-%s", s.Type, s.Start, s.End))
	}
	return fmt.Sprintf("%s: %s", d.WorkedDate, strings.Join(parts, ", "))
}

func LaborHours(spans []SpanSummary) float64 {
	total := 0.0
	for _, s := range spans {
		if strings.ToLower(s.Type) != SpanTypeLabor {
			continue
		}
		start, err := parseHHMM(s.Start)
		if err != nil {
			continue
		}
		end, err := parseHHMM(s.End)
		if err != nil {
			continue
		}
		total += float64(end-start) / 60.0
	}
	return total
}

func parseHHMM(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("must be HH:MM")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid hour")
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minute")
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("out of range")
	}
	return h*60 + m, nil
}

func extractDaySummary(detail map[string]any, fallbackDate string) DaySummary {
	summary := DaySummary{
		WorkedDate: fallbackDate,
		DidNotWork: anyToBool(detail["didNotWork"]),
		Spans:      []SpanSummary{},
	}
	if wd := anyToString(detail["workedDate"]); wd != "" {
		summary.WorkedDate = wd
	}

	spans, ok := anyToSlice(detail["timeEntrySpanDtos"])
	if !ok {
		return summary
	}

	for _, s := range spans {
		m, ok := anyToMap(s)
		if !ok {
			continue
		}
		start := tailTime(anyToString(m["startTimeStr"]))
		end := tailTime(anyToString(m["endTimeStr"]))
		typ := strings.ToLower(anyToString(m["timeEntrySpanType"]))
		if typ == "" {
			typ = SpanTypeLabor
		}
		summary.Spans = append(summary.Spans, SpanSummary{Type: typ, Start: start, End: end})
	}

	sort.Slice(summary.Spans, func(i, j int) bool {
		return summary.Spans[i].Start < summary.Spans[j].Start
	})
	return summary
}

func buildSpanDTOs(targetMDY string, spans []Span) []any {
	out := make([]any, 0, len(spans))
	for _, s := range spans {
		spanType := "Labor"
		paidBreak := any(nil)
		if s.Type == SpanTypeLunch {
			spanType = "Lunch"
			paidBreak = false
		}

		entry := map[string]any{
			"startTimeStr":    fmt.Sprintf("%s %s", targetMDY, s.Start),
			"endTimeStr":      fmt.Sprintf("%s %s", targetMDY, s.End),
			"timeEntrySpanType": spanType,
			"id":              0,
			"timeEntryId":     0,
			"paidBreak":       paidBreak,
			"source":          nil,
			"leaveType":       nil,
			"leaveTypeId":     nil,
			"leaveRequestId":  nil,
			"fullDayOff":      nil,
		}
		out = append(out, entry)
	}
	return out
}

func ensureTopLevel(metadata map[string]any, weekStart, weekEnd time.Time) {
	weekStartMDY := FormatMDY(weekStart)
	weekEndMDY := FormatMDY(weekEnd)

	if _, ok := metadata["id"]; !ok {
		metadata["id"] = 0
	}
	if _, ok := metadata["type"]; !ok {
		metadata["type"] = "TIME"
	}
	if _, ok := metadata["bypassLeaveValidation"]; !ok {
		metadata["bypassLeaveValidation"] = false
	}
	if _, ok := metadata["attachments"]; !ok || metadata["attachments"] == nil {
		metadata["attachments"] = []any{}
	}
	if _, ok := metadata["selectedDate"]; !ok || strings.TrimSpace(anyToString(metadata["selectedDate"])) == "" {
		metadata["selectedDate"] = weekStartMDY
	}
	if _, ok := metadata["selectedEndDate"]; !ok || strings.TrimSpace(anyToString(metadata["selectedEndDate"])) == "" {
		metadata["selectedEndDate"] = weekEndMDY
	}
	if _, ok := metadata["periodEndDate"]; !ok || strings.TrimSpace(anyToString(metadata["periodEndDate"])) == "" {
		metadata["periodEndDate"] = weekEndMDY
	}
	if _, ok := metadata["requisitionId"]; !ok {
		if engagementID, ok := anyToInt64(metadata["engagementId"]); ok {
			metadata["requisitionId"] = engagementID
		}
	}
}

func containsLunch(spans []Span) bool {
	for _, s := range spans {
		if s.Type == SpanTypeLunch {
			return true
		}
	}
	return false
}

func tailTime(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func deepCopyMap(in map[string]any) (map[string]any, error) {
	buf, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(buf, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func anyToMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func anyToSlice(v any) ([]any, bool) {
	s, ok := v.([]any)
	return s, ok
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

func anyToBool(v any) bool {
	b, ok := v.(bool)
	if ok {
		return b
	}
	return false
}

func anyToInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case float32:
		return int64(t), true
	case int:
		return int64(t), true
	case int64:
		return t, true
	case json.Number:
		i, err := t.Int64()
		if err == nil {
			return i, true
		}
		return 0, false
	default:
		return 0, false
	}
}
