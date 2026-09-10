package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"
	"github.com/steevin/neuron-cli/internal/notes"
	webui "github.com/steevin/neuron-cli/internal/web"
)

func newWebCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "web", Short: "Open your local Markdown workspace in the browser", Args: cobra.NoArgs,
		Long:    "Start a local browser workspace to browse, search, create and edit notes,\nfollow wikilinks and explore the graph. Uses the same Markdown vault as the CLI.\nEditing modes: Write edits Markdown source; Read shows the formatted result;\nBoth displays the editor and preview. A visual WYSIWYG editor is not yet available.\nThe server listens on 127.0.0.1 only. The printed session URL grants access.\nKeep this command running; press Ctrl+C to stop. No web hosting or Node.js required.",
		Example: "  neuron web\n  neuron web --port 7878\n  neuron web --vault /absolute/path/to/vault --no-open",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			vault, _ := cmd.Flags().GetString("vault")
			if vault == "" {
				vault = cfg.VaultPath
			}
			store, err := notes.NewStore(vault)
			if err != nil {
				return err
			}
			port, _ := cmd.Flags().GetInt("port")
			if port < 0 || port > 65535 {
				return fmt.Errorf("port must be between 0 and 65535")
			}
			listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			if err != nil {
				return err
			}
			defer listener.Close()
			app, err := webui.New(store.VaultPath, listener.Addr().String())
			if err != nil {
				return err
			}
			defer app.Close()
			server := &http.Server{Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			finished := make(chan error, 1)
			go func() { finished <- server.Serve(listener) }()
			fmt.Fprintf(cmd.OutOrStdout(), "\nNeuron web · %s\n%s\n\nKeep this terminal open. Press Ctrl+C to stop.\n", store.VaultPath, app.URL())
			noOpen, _ := cmd.Flags().GetBool("no-open")
			if !noOpen {
				if err := openExternal(app.URL()); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Could not open browser: %v\nOpen the session URL above manually.\n", err)
				}
			}
			select {
			case err := <-finished:
				if err != http.ErrServerClosed {
					return err
				}
				return nil
			case <-ctx.Done():
				shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := server.Shutdown(shutdown); err != nil {
					server.Close()
					return err
				}
				return nil
			}
		},
	}
	cmd.Flags().String("vault", "", "Override the configured vault for this session")
	cmd.Flags().Int("port", 0, "Local port (0 chooses an available port)")
	cmd.Flags().Bool("no-open", false, "Print the session URL without opening the browser")
	return cmd
}
func init() { rootCmd.AddCommand(newWebCommand()) }
