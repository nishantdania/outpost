package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nishantdania/outpost/internal/remote"
	"github.com/spf13/cobra"
)

func TestResolveRemoteRejectsUnavailableOutposts(t *testing.T) {
	for _, result := range []struct {
		status int
		body   string
	}{
		{http.StatusOK, `{"status":"stopped","guest_ip":"172.30.0.2"}`},
		{http.StatusOK, `{"status":"running","guest_ip":"not-an-ip"}`},
		{http.StatusNotFound, `{"error":"not found"}`},
		{http.StatusUnauthorized, `{"error":"unauthorized"}`},
	} {
		t.Run(result.body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(result.status)
				_, _ = w.Write([]byte(result.body))
			}))
			defer server.Close()
			dir, runner := t.TempDir(), &recordingRunner{}
			options := &rootOptions{serverURL: server.URL, ssh: remote.Config{User: "root", IdentityFile: dir + "/id", KnownHostsFile: dir + "/known"}, runner: runner}
			command := &cobra.Command{}
			command.SetContext(context.Background())
			if _, err := resolveRemote(command, options, "work"); err == nil {
				t.Fatal("resolved unavailable Outpost")
			}
			if runner.name != "" {
				t.Fatal("runner was invoked")
			}
		})
	}
}
