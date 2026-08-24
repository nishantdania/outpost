package doctor

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type fake struct {
	files    map[string]fs.FileInfo
	data     map[string][]byte
	commands map[string]bool
}

func (f fake) Stat(p string) (fs.FileInfo, error) {
	if v := f.files[p]; v != nil {
		return v, nil
	}
	return nil, errors.New("missing")
}
func (f fake) Open(p string) (io.ReadCloser, error) {
	if v, ok := f.data[p]; ok {
		return io.NopCloser(strings.NewReader(string(v))), nil
	}
	return nil, errors.New("missing")
}
func (f fake) ReadFile(p string) ([]byte, error) {
	if v, ok := f.data[p]; ok {
		return v, nil
	}
	return nil, errors.New("missing")
}
func (f fake) LookPath(p string) (string, error) {
	if f.commands[p] {
		return p, nil
	}
	return "", errors.New("missing")
}

type info struct {
	mode fs.FileMode
	uid  uint32
	gid  uint32
}

func (i info) Name() string       { return "x" }
func (i info) Size() int64        { return 0 }
func (i info) Mode() fs.FileMode  { return i.mode }
func (i info) ModTime() time.Time { return time.Time{} }
func (i info) IsDir() bool        { return i.mode.IsDir() }
func (i info) Sys() any           { return &syscall.Stat_t{Uid: i.uid, Gid: i.gid} }
func TestLocalFailsForMissingFiles(t *testing.T) {
	if !Local(fake{}, "key", "hosts").Failed() {
		t.Fatal("expected failure")
	}
}
func TestLocalAcceptsSecureRequirements(t *testing.T) {
	f := fake{files: map[string]fs.FileInfo{"key": info{mode: 0600}, "hosts": info{mode: 0644}}, commands: map[string]bool{"ssh": true, "scp": true, "rsync": true}}
	if Local(f, "key", "hosts").Failed() {
		t.Fatal("unexpected failure")
	}
}
func TestServerChecksContentAndDevices(t *testing.T) {
	f := fake{files: map[string]fs.FileInfo{"/dev/kvm": info{mode: fs.ModeCharDevice}, "/dev/net/tun": info{mode: fs.ModeCharDevice}, "/dev/userfaultfd": info{mode: fs.ModeCharDevice}, "/sys/fs/cgroup": info{mode: fs.ModeDir}, "/s": info{mode: fs.ModeDir}, "/j": info{mode: fs.ModeDir}, "/r": info{mode: fs.ModeDir}, "/sys/class/net/eth0": info{mode: fs.ModeDir}}, data: map[string][]byte{"/proc/sys/net/ipv4/ip_forward": []byte("1\n"), "/sys/fs/cgroup/cgroup.controllers": []byte("cpu")}, commands: map[string]bool{}}
	r := Server(f, ServerConfig{StateDir: "/s", JailerDir: "/j", RuntimeDir: "/r", Uplink: "eth0"})
	if !r.Failed() {
		t.Fatal("missing commands must fail")
	}
}

type httpFake struct{ status int }

func (h httpFake) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: h.status, Status: http.StatusText(h.status), Body: io.NopCloser(strings.NewReader(""))}, nil
}

func TestAPIRequiresExactOK(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://example.test/v1/outposts", nil)
	for _, status := range []int{http.StatusOK, http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError} {
		got := API(httpFake{status}, r)
		if got.OK != (status == http.StatusOK) {
			t.Fatalf("status %d: %+v", status, got)
		}
	}
}

type fullFake struct {
	fake
	accessErr   error
	hardlinkErr error
	runErr      error
}

func (f fullFake) LookupUser(name string) (*user.User, error) {
	if name == "root" {
		return &user.User{Uid: "0"}, nil
	}
	if name == "outpostd" {
		return &user.User{Uid: "10"}, nil
	}
	if name == "outpostvm" {
		return &user.User{Uid: "11"}, nil
	}
	return nil, errors.New("missing")
}
func (f fullFake) LookupGroup(name string) (*user.Group, error) {
	if name == "outpostd" {
		return &user.Group{Gid: "10"}, nil
	}
	if name == "outpostvm" {
		return &user.Group{Gid: "11"}, nil
	}
	return nil, errors.New("missing")
}
func (f fullFake) Access(string) error           { return f.accessErr }
func (f fullFake) Hardlink(string, string) error { return f.hardlinkErr }
func (f fullFake) Run(string, ...string) error   { return f.runErr }
func passingServer() (fullFake, ServerConfig) {
	commands := map[string]bool{}
	for _, n := range []string{"ip", "nft", "e2fsck", "resize2fs", "debugfs", "ssh", "scp", "rsync", "id"} {
		commands[n] = true
	}
	files := map[string]fs.FileInfo{"/dev/kvm": info{mode: fs.ModeCharDevice}, "/dev/net/tun": info{mode: fs.ModeCharDevice}, "/dev/userfaultfd": info{mode: fs.ModeCharDevice}, "/sys/fs/cgroup": info{mode: fs.ModeDir}, "/s": info{mode: fs.ModeDir | 0750, uid: 10, gid: 10}, "/j": info{mode: fs.ModeDir | 0750, gid: 11}, "/r": info{mode: fs.ModeDir | 0750, gid: 10}, "/sys/class/net/eth0": info{mode: fs.ModeDir}, "/unit": info{mode: 0644}, "/asset": info{mode: 0640}}
	data := map[string][]byte{"/proc/sys/net/ipv4/ip_forward": []byte("1\n"), "/sys/fs/cgroup/cgroup.controllers": []byte("cpu"), "/asset": []byte("a")}
	f := fullFake{fake: fake{files: files, data: data, commands: commands}}
	c := ServerConfig{StateDir: "/s", JailerDir: "/j", RuntimeDir: "/r", Uplink: "eth0", Users: []string{"outpostd", "outpostvm"}, Groups: []string{"outpostd", "outpostvm"}, Directories: []Directory{{Path: "/s", Mode: 0750, User: "outpostd", Group: "outpostd"}, {Path: "/j", Mode: 0750, User: "root", Group: "outpostvm"}, {Path: "/r", Mode: 0750, User: "root", Group: "outpostd"}}, Units: []string{"/unit"}, Assets: map[string]string{"asset": "/asset"}, Manifest: map[string]string{"asset": "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb"}}
	return f, c
}
func TestServerCompleteOfflinePass(t *testing.T) {
	f, c := passingServer()
	if r := Server(f, c); r.Failed() {
		t.Fatalf("%+v", r.Checks)
	}
}
func TestServerTargetedFailures(t *testing.T) {
	for _, kind := range []string{"ip-forward", "cgroup", "char-access", "user", "userfaultfd", "hardlink", "asset"} {
		f, c := passingServer()
		switch kind {
		case "ip-forward":
			f.data["/proc/sys/net/ipv4/ip_forward"] = []byte("0\n")
		case "cgroup":
			f.data["/sys/fs/cgroup/cgroup.controllers"] = nil
		case "char-access":
			f.accessErr = errors.New("denied")
		case "user":
			c.Users = []string{"missing"}
		case "userfaultfd":
			delete(f.files, "/dev/userfaultfd")
		case "hardlink":
			f.hardlinkErr = errors.New("cross-device")
		case "asset":
			f.data["/asset"] = []byte("bad")
		}
		report := Server(f, c)
		if !report.Failed() {
			t.Fatal(kind)
		}
		if kind == "userfaultfd" {
			found := false
			for _, check := range report.Checks {
				if check.Name == "userfaultfd" && !check.OK {
					found = true
				}
			}
			if !found {
				t.Fatal(kind)
			}
		}
	}
}
func TestServerOnlineSocketOwnerAndSystemdFailures(t *testing.T) {
	f, c := passingServer()
	c.Online = true
	c.Socket = "/socket"
	c.SocketUser = "root"
	c.SocketGroup = "outpostd"
	f.files["/socket"] = info{mode: fs.ModeSocket | 0660, gid: 10}
	if Server(f, c).Failed() {
		t.Fatal("passing online server failed")
	}
	f.runErr = errors.New("inactive")
	if !Server(f, c).Failed() {
		t.Fatal("inactive service passed")
	}
	f.runErr = nil
	f.files["/socket"] = info{mode: fs.ModeSocket | 0660, gid: 11}
	if !Server(f, c).Failed() {
		t.Fatal("wrong socket group passed")
	}
}
func TestServerAssetChecksAreStreamingAndSorted(t *testing.T) {
	f := fake{data: map[string][]byte{"/a": []byte("a"), "/b": []byte("b")}}
	r := Server(f, ServerConfig{Assets: map[string]string{"z": "/b", "a": "/a"}, Manifest: map[string]string{"a": "ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb", "z": "3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d"}})
	var got []string
	for _, c := range r.Checks {
		if c.Name == "a" || c.Name == "z" {
			got = append(got, c.Name)
			if !c.OK {
				t.Fatal(c)
			}
		}
	}
	if strings.Join(got, ",") != "a,z" {
		t.Fatal(got)
	}
}
func TestOptionalChecksDoNotFailAndRenderWarnings(t *testing.T) {
	report := Report{Checks: []Check{{Name: "optional", Optional: true, Detail: "missing"}}}
	if report.Failed() {
		t.Fatal("optional check failed report")
	}
	var text, json strings.Builder
	report.Text(&text)
	if !strings.Contains(text.String(), "warn") {
		t.Fatal(text.String())
	}
	if err := report.JSON(&json); err != nil || !strings.Contains(json.String(), `"optional":true`) {
		t.Fatalf("%v %s", err, json.String())
	}
}
func TestImageBuildChecksAbsentPartialAndValid(t *testing.T) {
	absent := imageBuildChecks(fake{})
	if len(absent) != 1 || absent[0].OK || !absent[0].Optional {
		t.Fatalf("%+v", absent)
	}
	partial := imageBuildChecks(fake{commands: map[string]bool{"podman": true}})
	if partial[len(partial)-1].OK {
		t.Fatal("partial podman passed")
	}
	files := map[string]fs.FileInfo{}
	for _, path := range []string{"/etc/outpostd/storage.conf", "/etc/outpostd/containers.conf"} {
		files[path] = info{mode: 0640}
	}
	for _, path := range []string{"/var/lib/outpostd/images", "/var/lib/outpostd/podman/storage", "/run/outpost/podman/storage"} {
		files[path] = info{mode: fs.ModeDir | 0750}
	}
	valid := fullFake{fake: fake{files: files, data: map[string][]byte{"/etc/subuid": []byte("outpostd:100000:65536\n"), "/etc/subgid": []byte("outpostd:200000:65536\n"), "/etc/outpostd/storage.conf": []byte("[storage]\n"), "/etc/outpostd/containers.conf": []byte("cgroup_manager = \"cgroupfs\"\n")}, commands: map[string]bool{"podman": true, "newuidmap": true, "newgidmap": true, "fuse-overlayfs": true}}}
	valid.files["/var/lib/outpostd/podman/storage"] = info{mode: fs.ModeDir | 0700}
	valid.files["/run/outpost/podman/storage"] = info{mode: fs.ModeDir | 0700}
	checks := imageBuildChecks(valid)
	if !checks[len(checks)-1].OK {
		t.Fatalf("%+v", checks)
	}
	found := map[string]bool{}
	for _, check := range checks {
		found[check.Name] = true
	}
	if !found["image-graphroot"] || !found["image-runroot"] {
		t.Fatalf("%+v", checks)
	}
	for _, data := range [][]byte{[]byte("outpostd:100000:65535\n"), []byte("outpostd:nope:65536\n")} {
		valid.data["/etc/subuid"] = data
		if imageBuildChecks(valid)[len(checks)-1].OK {
			t.Fatalf("accepted %q", data)
		}
	}
}

func TestOSProbeAccessDirectories(t *testing.T) {
	dir := t.TempDir()
	probe := OSProbe{}
	if err := probe.Access(dir); err != nil {
		t.Fatalf("writable directory: %v", err)
	}
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0700)
	if os.Geteuid() != 0 {
		if err := probe.Access(dir); err == nil {
			t.Fatal("read-only directory was writable")
		}
	}
}
func TestOSProbeAccessAcceptsCharacterDevice(t *testing.T) {
	if err := (OSProbe{}).Access("/dev/null"); err != nil {
		t.Fatal(err)
	}
}
func TestOSProbeAccessRejectsDirectorySymlink(t *testing.T) {
	outside := t.TempDir()
	link := filepath.Join(t.TempDir(), "images")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := (OSProbe{}).Access(link); err == nil {
		t.Fatal("symlink accepted")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside changed: %v %v", entries, err)
	}
}
func TestOSProbeHardlink(t *testing.T) {
	d := t.TempDir()
	j := t.TempDir()
	if err := (OSProbe{}).Hardlink(d, j); err != nil {
		t.Fatal(err)
	}
}

type onlineFake struct{ fake }

func (onlineFake) Run(string, ...string) error   { return nil }
func (onlineFake) Access(string) error           { return nil }
func (onlineFake) Hardlink(string, string) error { return nil }
func TestServerOnlineChecksUnitsAndSocket(t *testing.T) {
	f := onlineFake{fake{files: map[string]fs.FileInfo{"/socket": info{mode: fs.ModeSocket | 0660}, "/unit": info{mode: 0644}}, data: map[string][]byte{}, commands: map[string]bool{}}}
	r := Server(f, ServerConfig{Online: true, Socket: "/socket", Units: []string{"/unit"}})
	found := false
	for _, c := range r.Checks {
		if c.Name == "systemd-unit" && c.OK {
			found = true
		}
	}
	if !found {
		t.Fatal("missing online systemd check")
	}
}
func TestServerOfflineDoesNotRequireSocket(t *testing.T) {
	f := fake{}
	r := Server(f, ServerConfig{})
	for _, c := range r.Checks {
		if c.Name == "launcher-socket" {
			t.Fatal("offline socket check")
		}
	}
}
