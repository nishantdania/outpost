package remote

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

type Config struct {
	User, ProxyJump, IdentityFile, KnownHostsFile string
	AgentForwarding                               bool
}
type IO struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}
type Runner interface {
	Run(context.Context, string, []string, IO) error
}
type systemRunner struct{}

func (systemRunner) Run(ctx context.Context, name string, args []string, streams IO) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin, command.Stdout, command.Stderr = streams.Stdin, streams.Stdout, streams.Stderr
	return command.Run()
}
func SystemRunner() Runner { return systemRunner{} }
func DefaultConfig(home string) Config {
	base := filepath.Join(home, ".config", "outpost")
	return Config{User: "root", IdentityFile: filepath.Join(base, "keys", "id_ed25519"), KnownHostsFile: filepath.Join(base, "known_hosts")}
}

func validPath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && path != string(filepath.Separator)
}
func regular(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	return nil
}
func (c Config) Validate() error {
	if strings.TrimSpace(c.User) == "" || strings.ContainsAny(c.User, " @:\x00\r\n") {
		return fmt.Errorf("invalid SSH user")
	}
	if !validPath(c.IdentityFile) || !validPath(c.KnownHostsFile) {
		return fmt.Errorf("SSH identity and known_hosts paths must be absolute clean non-root paths")
	}
	if err := validateJump(c.ProxyJump); err != nil {
		return err
	}
	for _, path := range []string{c.IdentityFile, c.KnownHostsFile} {
		if err := regular(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("invalid SSH path %q: %w", path, err)
		}
	}
	return nil
}
func validateJump(value string) error {
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "-") || strings.IndexFunc(value, unicode.IsSpace) >= 0 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid SSH proxy jump")
	}
	host := value
	if at := strings.LastIndexByte(host, '@'); at >= 0 {
		if at == 0 || strings.Count(host, "@") != 1 {
			return fmt.Errorf("invalid SSH proxy jump")
		}
		host = host[at+1:]
	}
	if strings.HasPrefix(host, "-") {
		return fmt.Errorf("invalid SSH proxy jump")
	}
	if host == "" {
		return fmt.Errorf("invalid SSH proxy jump")
	}
	if strings.HasPrefix(host, "[") {
		end := strings.IndexByte(host, ']')
		if end < 2 || (end+1 < len(host) && host[end+1] != ':') || net.ParseIP(host[1:end]) == nil {
			return fmt.Errorf("invalid SSH proxy jump")
		}
		if end+1 < len(host) {
			return validPort(host[end+2:])
		}
		return nil
	}
	parts := strings.Split(host, ":")
	if len(parts) > 2 || parts[0] == "" {
		return fmt.Errorf("invalid SSH proxy jump")
	}
	if len(parts) == 2 {
		if err := validPort(parts[1]); err != nil {
			return err
		}
	}
	if net.ParseIP(parts[0]) != nil {
		return nil
	}
	for _, label := range strings.Split(parts[0], ".") {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("invalid SSH proxy jump")
		}
		for _, r := range label {
			if !(r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
				return fmt.Errorf("invalid SSH proxy jump")
			}
		}
	}
	return nil
}
func validPort(value string) error {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid SSH proxy jump")
	}
	return nil
}
func (c Config) Prepare() error {
	if err := c.Validate(); err != nil {
		return err
	}
	for _, path := range []string{filepath.Dir(c.IdentityFile), filepath.Dir(c.KnownHostsFile)} {
		if err := os.MkdirAll(path, 0700); err != nil {
			return fmt.Errorf("create SSH configuration directory: %w", err)
		}
	}
	if err := regular(c.KnownHostsFile); os.IsNotExist(err) {
		file, openErr := os.OpenFile(c.KnownHostsFile, os.O_CREATE|os.O_EXCL, 0600)
		if openErr != nil {
			return fmt.Errorf("open SSH known_hosts: %w", openErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			return fmt.Errorf("close SSH known_hosts: %w", closeErr)
		}
	} else if err != nil {
		return fmt.Errorf("invalid SSH known_hosts: %w", err)
	}
	return os.Chmod(c.KnownHostsFile, 0600)
}
func (c Config) EnsureIdentity(ctx context.Context, runner Runner) (string, error) {
	if err := c.Prepare(); err != nil {
		return "", err
	}
	public := c.IdentityFile + ".pub"
	if err := regular(c.IdentityFile); os.IsNotExist(err) {
		temporary := c.IdentityFile + ".new"
		defer os.Remove(temporary)
		defer os.Remove(temporary + ".pub")
		if err := runner.Run(ctx, "ssh-keygen", []string{"-q", "-t", "ed25519", "-N", "", "-f", temporary}, IO{}); err != nil {
			return "", fmt.Errorf("generate SSH identity: %w", err)
		}
		if err := os.Chmod(temporary, 0600); err != nil {
			return "", err
		}
		if err := os.Chmod(temporary+".pub", 0600); err != nil {
			return "", err
		}
		if err := os.Rename(temporary, c.IdentityFile); err != nil {
			return "", fmt.Errorf("install SSH identity: %w", err)
		}
		if err := os.Rename(temporary+".pub", public); err != nil {
			return "", fmt.Errorf("install SSH public key: %w", err)
		}
	} else if err != nil {
		return "", fmt.Errorf("invalid SSH identity: %w", err)
	} else if err := ensurePublic(ctx, runner, c.IdentityFile, public); err != nil {
		return "", err
	}
	if err := regular(public); err != nil {
		return "", fmt.Errorf("invalid SSH public key: %w", err)
	}
	if err := os.Chmod(c.IdentityFile, 0600); err != nil {
		return "", err
	}
	if err := os.Chmod(public, 0600); err != nil {
		return "", err
	}
	key, err := os.ReadFile(public)
	if err != nil {
		return "", fmt.Errorf("read SSH public key: %w", err)
	}
	if strings.TrimSpace(string(key)) == "" {
		return "", fmt.Errorf("SSH public key is empty")
	}
	return strings.TrimSpace(string(key)), nil
}
func ensurePublic(ctx context.Context, runner Runner, identity, public string) error {
	if err := regular(public); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("invalid SSH public key: %w", err)
	}
	temporary := public + ".new"
	defer os.Remove(temporary)
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	runErr := runner.Run(ctx, "ssh-keygen", []string{"-y", "-f", identity}, IO{Stdout: file})
	closeErr := file.Close()
	if runErr != nil {
		return fmt.Errorf("derive SSH public key: %w", runErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if err := regular(temporary); err != nil {
		return err
	}
	return os.Rename(temporary, public)
}
func (c Config) sshOptions() []string {
	args := []string{"-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile=" + c.KnownHostsFile, "-o", "IdentitiesOnly=yes", "-i", c.IdentityFile}
	if c.ProxyJump != "" {
		args = append(args, "-J", c.ProxyJump)
	}
	if c.AgentForwarding {
		args = append(args, "-A")
	}
	return args
}
func (c Config) destination(ip string) (string, error) {
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid guest IP %q", ip)
	}
	if strings.Contains(ip, ":") {
		ip = "[" + ip + "]"
	}
	return c.User + "@" + ip, nil
}
func (c Config) SSHArgs(ip string, remote []string, terminal bool) ([]string, error) {
	destination, err := c.destination(ip)
	if err != nil {
		return nil, err
	}
	args := c.sshOptions()
	if terminal {
		args = append(args, "-t")
	}
	args = append(args, destination)
	if len(remote) > 0 {
		for _, value := range remote {
			if strings.ContainsAny(value, "\x00\r\n") {
				return nil, fmt.Errorf("remote command contains control characters")
			}
		}
		args = append(args, Quote(remote))
	}
	return args, nil
}
func Quote(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(quoted, " ")
}
func (c Config) SCPArgs(ip, local, remote string, upload, recursive bool) ([]string, error) {
	destination, err := c.destination(ip)
	if err != nil {
		return nil, err
	}
	args := c.sshOptions()
	if recursive {
		args = append(args, "-r")
	}
	args = append(args, "--")
	guest := destination + ":" + remote
	if upload {
		return append(args, local, guest), nil
	}
	return append(args, guest, local), nil
}
func (c Config) RsyncArgs(ip, local, remote string, upload bool) ([]string, error) {
	destination, err := c.destination(ip)
	if err != nil {
		return nil, err
	}
	transport := append([]string{"ssh"}, c.sshOptions()...)
	args := []string{"-a", "-s", "-e", Quote(transport), "--"}
	guest := destination + ":" + remote
	if upload {
		return append(args, local, guest), nil
	}
	return append(args, guest, local), nil
}
