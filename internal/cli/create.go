package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type createOptions struct {
	Name      string
	VCPUs     int
	MemoryMiB int
	DiskGiB   int
}

func parseCreateArgs(args []string) (createOptions, error) {
	options := createOptions{VCPUs: 2, MemoryMiB: 4096, DiskGiB: 8}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--cpus" || arg == "--memory" || arg == "--disk" {
			if index+1 == len(args) {
				return options, fmt.Errorf("%s requires a value", arg)
			}
			value := args[index+1]
			index++
			var err error
			switch arg {
			case "--cpus":
				options.VCPUs, err = strconv.Atoi(value)
			case "--memory":
				options.MemoryMiB, err = parseMemory(value)
			case "--disk":
				options.DiskGiB, err = parseDisk(value)
			}
			if err != nil {
				return options, fmt.Errorf("invalid %s value %q", strings.TrimPrefix(arg, "--"), value)
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return options, fmt.Errorf("unknown option %q", arg)
		}
		if options.Name != "" {
			return options, fmt.Errorf("only one name may be specified")
		}
		options.Name = arg
	}
	return options, nil
}

func parseMemory(value string) (int, error) {
	value = strings.ToUpper(value)
	multiplier := 1
	if strings.HasSuffix(value, "G") || strings.HasSuffix(value, "GB") {
		value = strings.TrimSuffix(strings.TrimSuffix(value, "B"), "G")
		multiplier = 1024
	} else if strings.HasSuffix(value, "M") || strings.HasSuffix(value, "MB") {
		value = strings.TrimSuffix(strings.TrimSuffix(value, "B"), "M")
	}
	number, err := strconv.Atoi(value)
	return number * multiplier, err
}

func parseDisk(value string) (int, error) {
	value = strings.ToUpper(value)
	value = strings.TrimSuffix(strings.TrimSuffix(value, "B"), "G")
	return strconv.Atoi(value)
}

func create(ctx context.Context, options createOptions, stdout io.Writer) error {
	base, err := daemonURL()
	if err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		Name      string `json:"name"`
		VCPUs     int    `json:"vcpus"`
		MemoryMiB int    `json:"memory_mib"`
		DiskGiB   int    `json:"disk_gib"`
	}{options.Name, options.VCPUs, options.MemoryMiB, options.DiskGiB})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/outposts", strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(response.Body)
		return fmt.Errorf("daemon returned %s: %s", response.Status, strings.TrimSpace(string(message)))
	}
	var body struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		VCPUs     int    `json:"vcpus"`
		MemoryMiB int    `json:"memory_mib"`
		DiskGiB   int    `json:"disk_gib"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Created %s (%s): %d vCPU, %d MiB RAM, %d GiB disk\n", body.Name, body.ID, body.VCPUs, body.MemoryMiB, body.DiskGiB)
	return nil
}
