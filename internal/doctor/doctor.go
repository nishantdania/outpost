package doctor

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Probe interface {
	Stat(string) (os.FileInfo, error)
	LookPath(string) (string, error)
	ReadFile(string) ([]byte, error)
}

type OSProbe struct{}

func (OSProbe) Stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (OSProbe) LookPath(name string) (string, error)         { return exec.LookPath(name) }
func (OSProbe) ReadFile(path string) ([]byte, error)         { return os.ReadFile(path) }
func (OSProbe) Open(path string) (io.ReadCloser, error)      { return os.Open(path) }
func (OSProbe) LookupUser(name string) (*user.User, error)   { return user.Lookup(name) }
func (OSProbe) LookupGroup(name string) (*user.Group, error) { return user.LookupGroup(name) }
func (OSProbe) Run(name string, args ...string) error        { return exec.Command(name, args...).Run() }
func (OSProbe) Access(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("symlink is not writable storage")
	}
	if !info.IsDir() {
		fd, err := unix.Open(path, unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		return unix.Close(fd)
	}
	dir, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(dir)
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	name := ".outpost-doctor-" + hex.EncodeToString(random[:])
	file, err := unix.Openat(dir, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	closeErr := unix.Close(file)
	removeErr := unix.Unlinkat(dir, name, 0)
	return errors.Join(closeErr, removeErr)
}
func (OSProbe) Hardlink(state, jail string) error {
	source, err := os.CreateTemp(state, ".outpost-hardlink-")
	if err != nil {
		return err
	}
	target := filepath.Join(jail, filepath.Base(source.Name()))
	defer os.Remove(source.Name())
	defer os.Remove(target)
	return os.Link(source.Name(), target)
}

type Check struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Optional bool   `json:"optional"`
	Detail   string `json:"detail"`
}

type Report struct {
	Checks []Check `json:"checks"`
}

func (r Report) Failed() bool {
	for _, check := range r.Checks {
		if !check.OK && !check.Optional {
			return true
		}
	}
	return false
}

func (r Report) JSON(w io.Writer) error { return json.NewEncoder(w).Encode(r) }

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

func API(client HTTPClient, request *http.Request) Check {
	response, err := client.Do(request)
	if err != nil {
		return Check{Name: "api-auth", Detail: err.Error()}
	}
	defer response.Body.Close()
	return Check{Name: "api-auth", OK: response.StatusCode == http.StatusOK, Detail: response.Status}
}

func (r Report) Text(w io.Writer) {
	for _, check := range r.Checks {
		status := "ok"
		if !check.OK {
			status = "fail"
			if check.Optional {
				status = "warn"
			}
		}
		fmt.Fprintf(w, "%-5s %-20s %s\n", status, check.Name, check.Detail)
	}
}

func Local(p Probe, identity, knownHosts string) Report {
	checks := []Check{command(p, "ssh"), command(p, "scp"), command(p, "rsync")}
	checks = append(checks, regular(p, "identity", identity, 0600), regular(p, "known-hosts", knownHosts, 0644))
	return Report{Checks: checks}
}

type Directory struct {
	Path  string
	Mode  os.FileMode
	User  string
	Group string
}

type ServerConfig struct {
	StateDir    string
	JailerDir   string
	RuntimeDir  string
	Assets      map[string]string
	Manifest    map[string]string
	Uplink      string
	Socket      string
	Units       []string
	Online      bool
	Users       []string
	Groups      []string
	Directories []Directory
	SocketUser  string
	SocketGroup string
}

func Server(p Probe, config ServerConfig) Report {
	checks := []Check{command(p, "ip"), command(p, "nft"), command(p, "e2fsck"), command(p, "resize2fs"), command(p, "debugfs"), command(p, "ssh"), command(p, "scp"), command(p, "rsync"), command(p, "id"), charDevice(p, "kvm", "/dev/kvm"), charDevice(p, "tun", "/dev/net/tun"), charDevice(p, "userfaultfd", "/dev/userfaultfd"), directory(p, "cgroup-v2", "/sys/fs/cgroup"), content(p, "cgroup-controllers", "/sys/fs/cgroup/cgroup.controllers", false), directory(p, "state", config.StateDir), directory(p, "jailer", config.JailerDir), directory(p, "runtime", config.RuntimeDir), content(p, "ip-forward", "/proc/sys/net/ipv4/ip_forward", true)}
	for _, dir := range config.Directories {
		checks = append(checks, secureDirectory(p, dir))
	}
	if config.Uplink != "" {
		checks = append(checks, directory(p, "uplink", "/sys/class/net/"+config.Uplink))
	}
	if config.Online && config.Socket != "" {
		checks = append(checks, socket(p, "launcher-socket", config.Socket, config.SocketUser, config.SocketGroup))
	}
	for _, name := range config.Users {
		checks = append(checks, identity(p, "user-"+name, name, true))
	}
	for _, name := range config.Groups {
		checks = append(checks, identity(p, "group-"+name, name, false))
	}
	if err := hardlink(p, config.StateDir, config.JailerDir); err != nil {
		checks = append(checks, Check{Name: "state-jail-hardlink", Detail: err.Error()})
	} else {
		checks = append(checks, Check{Name: "state-jail-hardlink", OK: true})
	}
	for _, unit := range config.Units {
		checks = append(checks, regular(p, "systemd-"+filepath.Base(unit), unit, 0644))
		if config.Online {
			checks = append(checks, systemd(p, filepath.Base(unit)))
		}
	}
	names := make([]string, 0, len(config.Assets))
	for name := range config.Assets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		checks = append(checks, checksum(p, name, config.Assets[name], config.Manifest[name]))
	}
	checks = append(checks, imageBuildChecks(p)...)
	return Report{Checks: checks}
}

func imageBuildChecks(p Probe) []Check {
	if _, err := p.LookPath("podman"); err != nil {
		return []Check{{Name: "image-build", Optional: true, Detail: "podman is not installed"}}
	}
	checks := []Check{{Name: "podman", OK: true, Optional: true, Detail: "command available"}}
	for _, name := range []string{"newuidmap", "newgidmap", "fuse-overlayfs"} {
		check := command(p, name)
		check.Optional = true
		checks = append(checks, check)
	}
	for _, path := range []string{"/etc/outpostd/storage.conf", "/etc/outpostd/containers.conf", "/var/lib/outpostd/images", "/var/lib/outpostd/podman/storage", "/run/outpost/podman/storage"} {
		check := regularOrDirectory(p, path)
		check.Optional = true
		checks = append(checks, check)
	}
	for _, path := range []string{"/etc/subuid", "/etc/subgid"} {
		check := subID(p, path)
		check.Optional = true
		checks = append(checks, check)
	}
	ok := true
	for _, check := range checks {
		if !check.OK {
			ok = false
		}
	}
	checks = append(checks, Check{Name: "image-build", OK: ok, Optional: true, Detail: "rootless image build prerequisites"})
	return checks
}
func imageCheckName(path string) string {
	switch path {
	case "/var/lib/outpostd/podman/storage":
		return "image-graphroot"
	case "/run/outpost/podman/storage":
		return "image-runroot"
	default:
		return "image-" + filepath.Base(path)
	}
}
func regularOrDirectory(p Probe, path string) Check {
	name := imageCheckName(path)
	info, err := p.Stat(path)
	if err != nil {
		return Check{Name: name, Detail: err.Error()}
	}
	if path == "/etc/outpostd/storage.conf" || path == "/etc/outpostd/containers.conf" {
		if !info.Mode().IsRegular() || info.Mode().Perm() > 0640 {
			return Check{Name: name, Detail: "unsafe file"}
		}
		data, err := p.ReadFile(path)
		want := "[storage]"
		if path == "/etc/outpostd/containers.conf" {
			want = `cgroup_manager = "cgroupfs"`
		}
		if err != nil || !strings.Contains(string(data), want) {
			return Check{Name: name, Detail: "configuration is incomplete"}
		}
	} else if !info.IsDir() || (path == "/var/lib/outpostd/images" && info.Mode().Perm() != 0750) || (path != "/var/lib/outpostd/images" && info.Mode().Perm() != 0700 && info.Mode().Perm() != 0750) {
		return Check{Name: name, Detail: "wrong directory mode"}
	}
	if strings.HasPrefix(path, "/var/lib/outpostd/") || strings.HasPrefix(path, "/run/outpost/") {
		if q, ok := p.(interface{ Access(string) error }); ok {
			if err := q.Access(path); err != nil {
				return Check{Name: name, Detail: err.Error()}
			}
		}
	}
	return Check{Name: name, OK: true, Detail: path}
}
func subID(p Probe, path string) Check {
	data, err := p.ReadFile(path)
	if err != nil {
		return Check{Name: "image-" + filepath.Base(path), Detail: err.Error()}
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Split(line, ":")
		if len(parts) != 3 || parts[0] != "outpostd" {
			continue
		}
		start, first := strconv.ParseUint(parts[1], 10, 64)
		size, second := strconv.ParseUint(parts[2], 10, 64)
		if first == nil && second == nil && start > 0 && size >= 65536 {
			return Check{Name: "image-" + filepath.Base(path), OK: true, Detail: path}
		}
	}
	return Check{Name: "image-" + filepath.Base(path), Detail: "outpostd needs a contiguous range of at least 65536"}
}
func command(p Probe, name string) Check {
	_, err := p.LookPath(name)
	return result(name, err, "command available")
}
func regular(p Probe, name, path string, max os.FileMode) Check {
	info, err := p.Stat(path)
	if err != nil {
		return result(name, err, path)
	}
	if !info.Mode().IsRegular() && name != "kvm" && name != "tun" {
		return Check{Name: name, Detail: path + " is not a regular file"}
	}
	if max != 0 && info.Mode().Perm() > max {
		return Check{Name: name, Detail: path + " permissions are too broad"}
	}
	return Check{Name: name, OK: true, Detail: path}
}
func exists(p Probe, name, path string) Check { _, err := p.Stat(path); return result(name, err, path) }
func charDevice(p Probe, name, path string) Check {
	info, err := p.Stat(path)
	if err != nil {
		return result(name, err, path)
	}
	if info.Mode()&os.ModeCharDevice == 0 {
		return Check{Name: name, Detail: path + " is not a character device"}
	}
	if q, ok := p.(interface{ Access(string) error }); ok {
		return result(name, q.Access(path), path)
	}
	return Check{Name: name, Detail: "access probe unavailable"}
}
func socket(p Probe, name, path, account, group string) Check {
	info, err := p.Stat(path)
	if err != nil {
		return result(name, err, path)
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() > 0660 {
		return Check{Name: name, Detail: path + " has unsafe type or mode"}
	}
	if (account != "" || group != "") && !ownerMatches(p, info, account, group) {
		return Check{Name: name, Detail: path + " has wrong owner/group"}
	}
	return Check{Name: name, OK: true, Detail: path}
}
func content(p Probe, name, path string, one bool) Check {
	data, err := p.ReadFile(path)
	if err != nil {
		return result(name, err, path)
	}
	ok := len(data) > 0
	if one {
		ok = string(data) == "1\n" || string(data) == "1"
	}
	return Check{Name: name, OK: ok, Detail: path}
}
func identity(p Probe, label, name string, account bool) Check {
	if q, ok := p.(interface {
		LookupUser(string) (*user.User, error)
	}); ok && account {
		_, err := q.LookupUser(name)
		return result(label, err, name)
	}
	if q, ok := p.(interface {
		LookupGroup(string) (*user.Group, error)
	}); ok && !account {
		_, err := q.LookupGroup(name)
		return result(label, err, name)
	}
	return Check{Name: label, Detail: "identity probe unavailable"}
}
func systemd(p Probe, unit string) Check {
	q, ok := p.(interface{ Run(string, ...string) error })
	if !ok {
		return Check{Name: "systemd-" + unit, Detail: "command probe unavailable"}
	}
	err := q.Run("systemctl", "is-active", "--quiet", unit)
	if err == nil {
		err = q.Run("systemctl", "is-enabled", "--quiet", unit)
	}
	return result("systemd-"+unit, err, unit)
}
func secureDirectory(p Probe, want Directory) Check {
	info, err := p.Stat(want.Path)
	if err != nil {
		return result("directory-"+filepath.Base(want.Path), err, want.Path)
	}
	if !info.IsDir() || (want.Mode != 0 && info.Mode().Perm() != want.Mode) || !ownerMatches(p, info, want.User, want.Group) {
		return Check{Name: "directory-" + filepath.Base(want.Path), Detail: "wrong type, mode, or owner"}
	}
	return Check{Name: "directory-" + filepath.Base(want.Path), OK: true, Detail: want.Path}
}
func ownerMatches(p Probe, info os.FileInfo, wantUser, wantGroup string) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return wantUser == "" && wantGroup == ""
	}
	if wantUser != "" {
		q, ok := p.(interface {
			LookupUser(string) (*user.User, error)
		})
		if !ok {
			return false
		}
		u, err := q.LookupUser(wantUser)
		if err != nil || u.Uid != strconv.FormatUint(uint64(st.Uid), 10) {
			return false
		}
	}
	if wantGroup != "" {
		q, ok := p.(interface {
			LookupGroup(string) (*user.Group, error)
		})
		if !ok {
			return false
		}
		g, err := q.LookupGroup(wantGroup)
		if err != nil || g.Gid != strconv.FormatUint(uint64(st.Gid), 10) {
			return false
		}
	}
	return true
}
func hardlink(p Probe, state, jail string) error {
	if q, ok := p.(interface{ Hardlink(string, string) error }); ok {
		return q.Hardlink(state, jail)
	}
	a, err := p.Stat(state)
	if err != nil {
		return err
	}
	b, err := p.Stat(jail)
	if err != nil {
		return err
	}
	if !a.IsDir() || !b.IsDir() {
		return fmt.Errorf("state and jail must be directories")
	}
	return nil
}

func directory(p Probe, name, path string) Check {
	info, err := p.Stat(path)
	if err != nil {
		return result(name, err, path)
	}
	return Check{Name: name, OK: info.IsDir(), Detail: path}
}
func checksum(p Probe, name, path, want string) Check {
	if !sha256Pattern.MatchString(want) {
		return Check{Name: name, Detail: "missing pinned checksum"}
	}
	opener, ok := p.(interface {
		Open(string) (io.ReadCloser, error)
	})
	if !ok {
		return Check{Name: name, Detail: "stream probe unavailable"}
	}
	file, err := opener.Open(path)
	if err != nil {
		return result(name, err, path)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return result(name, err, path)
	}
	if hex.EncodeToString(hash.Sum(nil)) != want {
		return Check{Name: name, Detail: "checksum mismatch"}
	}
	return Check{Name: name, OK: true, Detail: path}
}
func result(name string, err error, detail string) Check {
	if err != nil {
		return Check{Name: name, Detail: detail + ": " + err.Error()}
	}
	return Check{Name: name, OK: true, Detail: detail}
}
