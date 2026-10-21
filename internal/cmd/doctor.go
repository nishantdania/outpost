package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/client"
	"github.com/nishantdania/outpost/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "doctor", Short: "Check local launcher connectivity", RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		connection, err := client.New()
		if err == nil {
			_, err = connection.ListOutposts(ctx)
		}
		check := doctor.Check{Name: "launcher", OK: err == nil, Detail: api.SocketPath}
		if err != nil {
			check.Detail = err.Error()
		}
		report := doctor.Report{Checks: []doctor.Check{check}}
		if jsonOutput {
			if err := report.JSON(cmd.OutOrStdout()); err != nil {
				return err
			}
		} else {
			report.Text(cmd.OutOrStdout())
		}
		if report.Failed() {
			return fmt.Errorf("doctor found failing checks")
		}
		return nil
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "write JSON")
	return command
}
