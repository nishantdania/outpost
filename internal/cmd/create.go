package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/ark"
	"github.com/nishantdania/ark/internal/client"
)

func newCreateCmd(options *rootOptions) *cobra.Command {
	input := client.CreateArkInput{ImageID: ark.DefaultImageID, VCPUs: ark.DefaultVCPUs, MemoryMiB: ark.DefaultMemoryMiB, DiskGiB: ark.DefaultDiskGiB}
	memory, disk, publicKeyPath := "4G", "8G", ""
	command := &cobra.Command{Use: "create <name>", Short: "Create an Ark", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		input.Name = args[0]
		if publicKeyPath != "" {
			key, err := os.ReadFile(publicKeyPath)
			if err != nil {
				return fmt.Errorf("read SSH public key: %w", err)
			}
			input.SSHPublicKey = strings.TrimSpace(string(key))
		} else {
			key, err := options.ssh.EnsureIdentity(cmd.Context(), options.runner)
			if err != nil {
				return err
			}
			input.SSHPublicKey = key
		}
		if input.MemoryMiB, err = parseMemoryMiB(memory); err != nil {
			return fmt.Errorf("parse memory: %w", err)
		}
		if input.DiskGiB, err = parseDiskGiB(disk); err != nil {
			return fmt.Errorf("parse disk: %w", err)
		}
		arkdClient, err := client.New(options.serverURL, options.token)
		if err != nil {
			return fmt.Errorf("create arkd client: %w", err)
		}
		result, err := arkdClient.CreateArkWith(cmd.Context(), input)
		if err != nil {
			return err
		}
		writer, err := newOutputWriter(options, cmd.OutOrStdout())
		if err != nil {
			return err
		}
		return writer.Write(result, arkTable([]api.Ark{result}))
	}}
	command.Flags().StringVar(&input.ImageID, "image", input.ImageID, "image ID")
	command.Flags().IntVar(&input.VCPUs, "cpus", input.VCPUs, "virtual CPUs")
	command.Flags().StringVar(&memory, "memory", memory, "memory (MiB, M, MB, G, or GB)")
	command.Flags().StringVar(&disk, "disk", disk, "disk (GiB, G, or GB)")
	command.Flags().StringVar(&publicKeyPath, "ssh-public-key", publicKeyPath, "SSH public key file")
	return command
}

func parseMemoryMiB(value string) (int, error) {
	return parseSize(value, map[string]int{"": 1, "M": 1, "MB": 1, "MIB": 1, "G": 1024, "GB": 1024, "GIB": 1024})
}
func parseDiskGiB(value string) (int, error) {
	return parseSize(value, map[string]int{"": 1, "G": 1, "GB": 1, "GIB": 1})
}
func parseSize(value string, units map[string]int) (int, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	number := strings.TrimRight(value, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	unit := strings.TrimPrefix(value, number)
	multiplier, ok := units[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported unit %q", unit)
	}
	parsed, err := strconv.Atoi(number)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("must be a positive size")
	}
	if parsed > int(^uint(0)>>1)/multiplier {
		return 0, fmt.Errorf("size is too large")
	}
	return parsed * multiplier, nil
}
