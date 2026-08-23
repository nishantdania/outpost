package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nishantdania/ark/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCmd(options *rootOptions) *cobra.Command {
	return newDoctorCmdWith(options, doctor.OSProbe{}, &http.Client{Timeout: 5 * time.Second})
}

func newDoctorCmdWith(options *rootOptions, probe doctor.Probe, client doctor.HTTPClient) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "doctor", Short: "Check local Ark client requirements", RunE: func(cmd *cobra.Command, args []string) error {
		report := doctor.Local(probe, options.ssh.IdentityFile, options.ssh.KnownHostsFile)
		request, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, options.serverURL+"/v1/arks", nil)
		if err == nil && options.token != "" {
			request.Header.Set("Authorization", "Bearer "+options.token)
		}
		if err != nil {
			report.Checks = append(report.Checks, doctor.Check{Name: "api-auth", Detail: err.Error()})
		} else {
			report.Checks = append(report.Checks, doctor.API(client, request))
		}
		if jsonOutput {
			err = report.JSON(cmd.OutOrStdout())
		} else {
			report.Text(cmd.OutOrStdout())
		}
		if err != nil {
			return err
		}
		if report.Failed() {
			return fmt.Errorf("doctor found failing checks")
		}
		return nil
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "write JSON")
	return command
}
