package timecard

import (
	"testing"
	"time"
)

func TestParseSpanArg(t *testing.T) {
	s, err := ParseSpanArg("labor:09:00-17:00")
	if err != nil {
		t.Fatalf("ParseSpanArg failed: %v", err)
	}
	if s.Type != SpanTypeLabor || s.Start != "09:00" || s.End != "17:00" {
		t.Fatalf("unexpected span: %+v", s)
	}
}

func TestValidateSpansOverlap(t *testing.T) {
	a, _ := ParseSpanArg("labor:09:00-12:00")
	b, _ := ParseSpanArg("lunch:11:30-12:30")
	if _, err := ValidateSpans([]Span{a, b}); err == nil {
		t.Fatalf("expected overlap validation error")
	}
}

func TestWeekStartMonday(t *testing.T) {
	loc := time.UTC
	d, _ := time.ParseInLocation("2006-01-02", "2026-02-18", loc) // Wednesday
	start := WeekStartMonday(d)
	if got := start.Format("2006-01-02"); got != "2026-02-16" {
		t.Fatalf("unexpected week start: %s", got)
	}
}

func TestPatchDayReplacesTargetOnly(t *testing.T) {
	metadata := map[string]any{
		"engagementId": float64(12345678),
		"timecardTemplateId": float64(4),
		"billingItemDetails": []any{
			map[string]any{"workedDate": "02/17/2026", "didNotWork": false, "timeEntrySpanDtos": nil, "timeEntry": map[string]any{}},
			map[string]any{"workedDate": "02/18/2026", "didNotWork": false, "timeEntrySpanDtos": nil, "timeEntry": map[string]any{}},
		},
	}

	loc := time.UTC
	target, _ := time.ParseInLocation("2006-01-02", "2026-02-18", loc)
	a, _ := ParseSpanArg("labor:09:00-12:00")
	b, _ := ParseSpanArg("lunch:12:00-12:30")
	c, _ := ParseSpanArg("labor:12:30-17:00")
	spans, err := ValidateSpans([]Span{a, b, c})
	if err != nil {
		t.Fatalf("validate spans failed: %v", err)
	}

	patched, change, err := PatchDay(metadata, target, spans, false)
	if err != nil {
		t.Fatalf("PatchDay failed: %v", err)
	}

	details := patched["billingItemDetails"].([]any)
	day1 := details[0].(map[string]any)
	day2 := details[1].(map[string]any)

	if day1["timeEntrySpanDtos"] != nil {
		t.Fatalf("non-target day should be unchanged")
	}
	if day2["timeEntrySpanDtos"] == nil {
		t.Fatalf("target day spans were not set")
	}
	if len(day2["timeEntrySpanDtos"].([]any)) != 3 {
		t.Fatalf("expected 3 spans on target day")
	}
	if !change.HadExisting && len(change.Existing.Spans) > 0 {
		t.Fatalf("change metadata inconsistent")
	}
}
