package cli

import (
	"context"
	"fmt"

	"github.com/ihildy/magnit-vms-hour-logging-cli/internal/auth"
	"github.com/ihildy/magnit-vms-hour-logging-cli/internal/config"
	"github.com/ihildy/magnit-vms-hour-logging-cli/internal/output"
	"github.com/ihildy/magnit-vms-hour-logging-cli/internal/timecard"

	"github.com/spf13/cobra"
)

func newMarkDNWCmd(app *App) *cobra.Command {
	var date string
	var engagementID int64
	var dryRun bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "mark-dnw --date YYYY-MM-DD",
		Short: "Mark a day as did-not-work",
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
			client, _, httpCtx, err := app.NewAuthedClient(ctx)
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

			patched, change, err := timecard.PatchDay(metadata, targetDate, nil, true)
			if err != nil {
				return err
			}

			if err := confirmConflict(app, change, yes); err != nil {
				return err
			}

			if dryRun {
				payload := map[string]any{
					"ok":            true,
					"operation":     "mark_dnw",
					"date":          date,
					"engagement_id": resolvedEngagement,
					"dry_run":       true,
					"change":        change,
					"payload":       patched,
				}
				human := "Dry run complete\n" + formatDayChangeHuman(change)
				return output.Write(app.Stdout, app.JSONOutput, human, payload)
			}

			xsrf, err := auth.ExtractXSRFToken(httpCtx.Auth.Client, app.BaseURL())
			if err != nil {
				return err
			}

			saveResp, err := client.SaveBillingItems(ctx, patched, xsrf)
			if err != nil {
				return err
			}
			if saveResp.Errors != nil || saveResp.BillingItemDetailErr != nil {
				return fmt.Errorf("save API returned validation errors")
			}

			totalHours, _ := client.GetTotalHours(ctx, resolvedEngagement, weekStartMDY)

			payload := map[string]any{
				"ok":              true,
				"operation":       "mark_dnw",
				"date":            date,
				"engagement_id":   resolvedEngagement,
				"dry_run":         false,
				"billing_item_id": saveResp.BillingItemID,
				"change":          change,
				"total_hours":     totalHours,
			}
			human := fmt.Sprintf("Marked %s as did-not-work (billingItemId=%d)", date, saveResp.BillingItemID)
			return output.Write(app.Stdout, app.JSONOutput, human, payload)
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Target date in YYYY-MM-DD")
	cmd.Flags().Int64Var(&engagementID, "engagement", 0, "Engagement ID override")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Validate and show payload diff without saving")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip interactive conflict confirmation")
	_ = cmd.MarkFlagRequired("date")
	return cmd
}
