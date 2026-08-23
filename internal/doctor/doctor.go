package doctor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"syscall"
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
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err == nil {
		err = file.Close()
	}
	return err
}
func (OSProbe) Hardlink(state, jail string) error {
	source, err := os.CreateTemp(state, ".ark-hardlink-")
	if err != nil {
		return err
	}
	target := filepath.Join(jail, filepath.Base(source.Name()))
	defer os.Remove(source.Name())
	defer os.Remove(target)
	return os.Link(source.Name(), target)
}

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type Report struct {
	Checks []Check `json:"checks"`
}

func (r Report) Failed() bool {
	for _, check := range r.Checks {
		if !check.OK {
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
	return Report{Checks: checks}
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
