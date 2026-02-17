package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ihildy/magnit-vms-hour-logging-cli/internal/auth"
	"github.com/ihildy/magnit-vms-hour-logging-cli/internal/keyring"
	"github.com/ihildy/magnit-vms-hour-logging-cli/internal/output"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newAuthCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication credentials",
	}
	cmd.AddCommand(newAuthLoginCmd(app))
	cmd.AddCommand(newAuthStatusCmd(app))
	cmd.AddCommand(newAuthLogoutCmd(app))
	return cmd
}

func newAuthLoginCmd(app *App) *cobra.Command {
	var username string
	var password string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store credentials in keychain and verify login",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if strings.TrimSpace(username) == "" {
				fmt.Fprint(app.Stderr, "Username: ")
				reader := bufio.NewReader(app.Stdin)
				line, err := reader.ReadString('\n')
				if err != nil && !errors.Is(err, io.EOF) {
					return err
				}
				username = strings.TrimSpace(line)
			}
			if username == "" {
				return fmt.Errorf("username is required")
			}

			password, err := resolvePassword(app, password)
			if err != nil {
				return err
			}

			httpClient, err := auth.NewHTTPClient()
			if err != nil {
				return err
			}
			authn := &auth.Authenticator{BaseURL: app.BaseURL(), Client: httpClient}
			if err := authn.Login(ctx, username, password); err != nil {
				return err
			}

			user, err := authn.CurrentUser(ctx)
			if err != nil {
				return err
			}

			if err := keyring.SaveCredentials(keyring.Credentials{Username: username, Password: password}); err != nil {
				return err
			}

			payload := map[string]any{
				"ok":        true,
				"operation": "auth_login",
				"username":  username,
				"user": map[string]any{
					"userId":   user["userId"],
					"fullName": user["fullName"],
					"email":    user["email"],
				},
			}
			human := fmt.Sprintf("Login successful for %s", username)
			return output.Write(app.Stdout, app.JSONOutput, human, payload)
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Account username (email)")
	cmd.Flags().StringVar(&password, "password", "", "Account password (non-interactive; avoid shell history leaks)")
	return cmd
}

func newAuthStatusCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check whether stored credentials can authenticate",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			creds, err := keyring.LoadCredentials()
			if err != nil {
				payload := map[string]any{"ok": true, "operation": "auth_status", "authenticated": false}
				return output.Write(app.Stdout, app.JSONOutput, "No stored credentials", payload)
			}

			httpClient, err := auth.NewHTTPClient()
			if err != nil {
				return err
			}
			authn := &auth.Authenticator{BaseURL: app.BaseURL(), Client: httpClient}
			if err := authn.Login(ctx, creds.Username, creds.Password); err != nil {
				payload := map[string]any{"ok": true, "operation": "auth_status", "authenticated": false, "reason": err.Error()}
				return output.Write(app.Stdout, app.JSONOutput, "Stored credentials are invalid", payload)
			}

			user, err := authn.CurrentUser(ctx)
			if err != nil {
				payload := map[string]any{"ok": true, "operation": "auth_status", "authenticated": false, "reason": err.Error()}
				return output.Write(app.Stdout, app.JSONOutput, "Stored credentials are invalid", payload)
			}

			payload := map[string]any{
				"ok":            true,
				"operation":     "auth_status",
				"authenticated": true,
				"username":      creds.Username,
				"user": map[string]any{
					"userId":   user["userId"],
					"fullName": user["fullName"],
					"email":    user["email"],
				},
			}
			return output.Write(app.Stdout, app.JSONOutput, "Authenticated", payload)
		},
	}
}

func newAuthLogoutCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete stored credentials from keychain",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := keyring.DeleteCredentials(); err != nil {
				return err
			}
			payload := map[string]any{"ok": true, "operation": "auth_logout"}
			return output.Write(app.Stdout, app.JSONOutput, "Credentials removed", payload)
		},
	}
}

func resolvePassword(app *App, provided string) (string, error) {
	if provided != "" {
		return provided, nil
	}

	stdinFile, ok := app.Stdin.(*os.File)
	if !ok {
		return "", fmt.Errorf("stdin is not a terminal file")
	}
	fmt.Fprint(app.Stderr, "Password: ")
	bytes, err := term.ReadPassword(int(stdinFile.Fd()))
	fmt.Fprintln(app.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	password := strings.TrimSpace(string(bytes))
	if password == "" {
		return "", fmt.Errorf("password is required")
	}
	return password, nil
}
