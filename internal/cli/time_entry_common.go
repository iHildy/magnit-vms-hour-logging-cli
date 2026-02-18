package cli

import (
	"fmt"
	"strings"

	"github.com/ihildy/magnit-vms-cli/internal/timecard"
)

func parseAndValidateSpans(raw []string) ([]timecard.Span, error) {
	spans := make([]timecard.Span, 0, len(raw))
	for _, item := range raw {
		span, err := timecard.ParseSpanArg(item)
		if err != nil {
			return nil, err
		}
		spans = append(spans, span)
	}
	return timecard.ValidateSpans(spans)
}

func formatDayChangeHuman(change timecard.DayChange) string {
	var b strings.Builder
	b.WriteString("Existing: ")
	b.WriteString(timecard.FormatDaySummaryHuman(change.Existing))
	b.WriteString("\n")
	b.WriteString("Proposed: ")
	b.WriteString(timecard.FormatDaySummaryHuman(change.Proposed))
	return b.String()
}

func confirmConflict(app *App, change timecard.DayChange, yes bool) error {
	if !change.HadExisting || yes {
		return nil
	}
	ok, err := app.PromptConfirm("Target day already has entries. Replace them?")
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("aborted by user")
	}
	return nil
}
