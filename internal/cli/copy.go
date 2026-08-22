package cli

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nishantdania/outpost/internal/config"
)

func copyArgs(args []string) (string, string, os.FileMode, error) {
	mode := os.FileMode(0o600)
	values := make([]string, 0, 2)
	for index := 0; index < len(args); index++ {
		if args[index] == "--mode" {
			if index+1 == len(args) {
				return "", "", 0, fmt.Errorf("--mode requires a value")
			}
			parsed, err := strconv.ParseUint(args[index+1], 8, 9)
			if err != nil || parsed > 0o777 {
				return "", "", 0, fmt.Errorf("invalid mode")
			}
			mode = os.FileMode(parsed)
			index++
			continue
		}
		values = append(values, args[index])
	}
	if len(values) != 2 {
		return "", "", 0, fmt.Errorf("usage: outpost copy [--mode MODE] <source> <outpost>:<destination>")
	}
	return values[0], values[1], mode, nil
}

func copyToOutpost(ctx context.Context, source, target string, mode os.FileMode, stdout io.Writer) error {
	separator := strings.IndexByte(target, ':')
	if separator < 1 || separator == len(target)-1 {
		return fmt.Errorf("target must be <outpost>:<destination>")
	}
	identifier, destination := target[:separator], target[separator+1:]
	if !filepath.IsAbs(destination) {
		return fmt.Errorf("destination must be an absolute path")
	}
	record, err := findOutpost(ctx, identifier)
	if err != nil {
		return err
	}
	if record.Status != "running" || record.IP == "" {
		return fmt.Errorf("outpost is not reachable")
	}
	cfg, err := config.LoadClient()
	if err != nil {
		return err
	}
	destination64 := base64.StdEncoding.EncodeToString([]byte(destination))
	guest := "set -eu; umask 077; destination=$(printf %s " + destination64 + " | base64 -d); parent=$(dirname -- \"$destination\"); mkdir -p -- \"$parent\"; temporary=$(mktemp \"$parent/.outpost-copy.XXXXXX\"); trap 'rm -f \"$temporary\"' EXIT; cat > \"$temporary\"; chmod " + fmt.Sprintf("%03o", mode) + " \"$temporary\"; mv -f -- \"$temporary\" \"$destination\"; trap - EXIT"
	inner := "ssh -o LogLevel=ERROR -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ~/.local/share/outpost/id_ed25519 root@" + record.IP + " " + shellQuote(guest)
	remote := inner
	localSource := source
	var input *os.File
	if strings.HasPrefix(source, "host:") {
		hostSource := strings.TrimPrefix(source, "host:")
		if hostSource == "" {
			return fmt.Errorf("host source is required")
		}
		if cfg.SSHHost == "local" {
			if strings.HasPrefix(hostSource, "~/") {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				hostSource = filepath.Join(home, strings.TrimPrefix(hostSource, "~/"))
			}
			localSource = hostSource
		} else {
			source64 := base64.StdEncoding.EncodeToString([]byte(hostSource))
			remote = "set -e -o pipefail; source=$(printf %s " + source64 + " | base64 -d); case \"$source\" in '~/'*) source=\"$HOME/${source#??}\";; esac; cat -- \"$source\" | " + inner
		}
	}
	if localSource != "-" && !(strings.HasPrefix(source, "host:") && cfg.SSHHost != "local") {
		input, err = os.Open(localSource)
		if err != nil {
			return err
		}
		defer input.Close()
	}
	var command *exec.Cmd
	if cfg.SSHHost == "local" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		key := filepath.Join(home, ".local", "share", "outpost", "id_ed25519")
		command = exec.CommandContext(ctx, "ssh", "-o", "LogLevel=ERROR", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-i", key, "root@"+record.IP, guest)
	} else {
		command = exec.CommandContext(ctx, "ssh", cfg.SSHHost, remote)
	}
	if source == "-" {
		command.Stdin = os.Stdin
	} else if input != nil {
		command.Stdin = input
	}
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Copied %s to %s\n", source, target)
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
