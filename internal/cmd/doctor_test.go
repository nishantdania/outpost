package cmd

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/doctor"
	"github.com/nishantdania/outpost/internal/remote"
)

type doctorHTTP struct{ status int }

func (d doctorHTTP) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: d.status, Status: http.StatusText(d.status), Body: io.NopCloser(strings.NewReader(""))}, nil
}

func TestDoctorCommandHTTPStatuses(t *testing.T) {
	d := t.TempDir()
	key := filepath.Join(d, "key")
	hosts := filepath.Join(d, "hosts")
	if err := os.WriteFile(key, []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hosts, []byte("hosts"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{200, 401, 404, 500} {
		o := &rootOptions{serverURL: "http://example.test", token: "token", ssh: remote.Config{IdentityFile: key, KnownHostsFile: hosts}}
		cmd := newDoctorCmdWith(o, doctor.OSProbe{}, doctorHTTP{status})
		var out strings.Builder
		cmd.SetOut(&out)
		err := cmd.Execute()
		if (err == nil) != (status == 200) {
			t.Fatalf("status %d error %v output %s", status, err, out.String())
		}
		if !strings.Contains(out.String(), http.StatusText(status)) {
			t.Fatalf("missing status %d: %s", status, out.String())
		}
	}
}
func TestDoctorCommandUsesConfiguredSSHFiles(t *testing.T) {
	o := &rootOptions{serverURL: "http://example.test", ssh: remote.Config{IdentityFile: "/chosen/key", KnownHostsFile: "/chosen/hosts"}}
	cmd := newDoctorCmdWith(o, doctor.OSProbe{}, doctorHTTP{200})
	var out strings.Builder
	cmd.SetOut(&out)
	_ = cmd.Execute()
	if !strings.Contains(out.String(), "/chosen/key") || !strings.Contains(out.String(), "/chosen/hosts") {
		t.Fatal(out.String())
	}
}
