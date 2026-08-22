package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nishantdania/outpost/internal/update"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func versions(ctx context.Context, v string, w io.Writer) error {
	fmt.Fprintf(w, "Local:  %s\n", v)
	return serverVersion(ctx, w)
}
func serverVersion(ctx context.Context, w io.Writer) error {
	r, e := request(ctx, http.MethodGet, "/version")
	if e != nil {
		return e
	}
	defer r.Body.Close()
	var b struct {
		Version string `json:"version"`
	}
	if e = json.NewDecoder(r.Body).Decode(&b); e != nil {
		return e
	}
	fmt.Fprintf(w, "Server: %s\n", b.Version)
	return nil
}
func localUpdate(ctx context.Context, v string, w io.Writer) error {
	x, e := os.Executable()
	if e != nil {
		return e
	}
	r, e := update.Apply(ctx, update.Options{Component: "outpost", CurrentVersion: v, Executable: x})
	if e != nil {
		return e
	}
	fmt.Fprintf(w, "Local:  %s → %s\n", r.CurrentVersion, r.LatestVersion)
	return nil
}
func serverUpdate(ctx context.Context, w io.Writer) error {
	r, e := request(ctx, http.MethodPost, "/update")
	if e != nil {
		return e
	}
	defer r.Body.Close()
	var b struct {
		CurrentVersion string `json:"current_version"`
		LatestVersion  string `json:"latest_version"`
	}
	if e = json.NewDecoder(r.Body).Decode(&b); e != nil {
		return e
	}
	fmt.Fprintf(w, "Server: %s → %s\n", b.CurrentVersion, b.LatestVersion)
	return nil
}
func serverUninstall(ctx context.Context, w io.Writer) error {
	r, e := request(ctx, http.MethodPost, "/uninstall")
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusNoContent {
		return fmt.Errorf("daemon returned %s", r.Status)
	}
	fmt.Fprintln(w, "Server: uninstalled")
	return nil
}
func localUninstall(w io.Writer) error {
	d, e := os.UserConfigDir()
	if e != nil {
		return e
	}
	if e = os.RemoveAll(filepath.Join(d, "outpost")); e != nil {
		return e
	}
	x, e := os.Executable()
	if e != nil {
		return e
	}
	if e = os.Remove(x); e != nil {
		return e
	}
	fmt.Fprintln(w, "Local:  uninstalled")
	return nil
}
