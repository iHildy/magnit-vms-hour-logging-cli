package cli

import (
	"context"
	"fmt"

	"github.com/ihildy/magnit-vms-hour-logging-cli/internal/config"
	"github.com/ihildy/magnit-vms-hour-logging-cli/internal/output"
	"github.com/ihildy/magnit-vms-hour-logging-cli/internal/timecard"

	"github.com/spf13/cobra"
)

func newShowCmd(app *App) *cobra.Command {
	var date string
	var engagementID int64

	cmd := &cobra.Command{
		Use:   "show --date YYYY-MM-DD",
		Short: "Show logged spans for a day",
		RunE: func(cmd *cobra.Command, args []string) error {
			if date == "" {
				return fmt.Errorf("--date is required")
			}
			loc, err := config.ResolveTimezone(app.Cfg)
			if err != nil {
				return err
			}
			targetDate, err := timecard.ParseDateYYYYMMDD(date, loc)
			if err != nil {
				return err
			}

			ctx := context.Background()
			client, _, _, err := app.NewAuthedClient(ctx)
			if err != nil {
				return err
			}

			resolvedEngagement, err := app.ResolveEngagementID(ctx, client, engagementID)
			if err != nil {
				return err
			}

			weekStartMDY := timecard.FormatMDY(timecard.WeekStartMonday(targetDate))
			metadata, err := client.GetMetadata(ctx, resolvedEngagement, weekStartMDY)
			if err != nil {
				return err
			}
			summary, err := timecard.FindDaySummary(metadata, targetDate)
			if err != nil {
				return err
			}

			totalHours, _ := client.GetTotalHours(ctx, resolvedEngagement, weekStartMDY)

			payload := map[string]any{
				"ok":            true,
				"operation":     "show",
				"engagement_id": resolvedEngagement,
				"date":          date,
				"week_start":    weekStartMDY,
				"summary":       summary,
				"total_hours":   totalHours,
			}
			human := timecard.FormatDaySummaryHuman(summary)
			return output.Write(app.Stdout, app.JSONOutput, human, payload)
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Target date in YYYY-MM-DD")
	cmd.Flags().Int64Var(&engagementID, "engagement", 0, "Engagement ID override")
	_ = cmd.MarkFlagRequired("date")
	return cmd
}
