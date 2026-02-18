package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/ihildy/magnit-vms-cli/internal/output"

	"github.com/spf13/cobra"
)

func newEngagementCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "engagement",
		Short: "Engagement discovery commands",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available engagements",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			client, user, _, err := app.NewAuthedClient(ctx)
			if err != nil {
				return err
			}

			items, err := client.GetEngagementItems(ctx, 0, 200)
			if err != nil {
				return err
			}

			payload := map[string]any{
				"ok":         true,
				"operation":  "engagement_list",
				"count":      len(items),
				"user_id":    user["userId"],
				"engagements": items,
			}

			if app.JSONOutput {
				return output.WriteJSON(app.Stdout, payload)
			}

			if len(items) == 0 {
				_, err := fmt.Fprintln(app.Stdout, "No engagements returned")
				return err
			}
			fmt.Fprintf(app.Stdout, "Found %d engagement(s):\n", len(items))
			for _, item := range items {
				buyer := strings.TrimSpace(item.BuyerName)
				if buyer == "" {
					buyer = "(unknown buyer)"
				}
				fmt.Fprintf(app.Stdout, "- %d  [%s]  %s\n", item.ID, item.Status, buyer)
			}
			return nil
		},
	})

	return cmd
}
