package outpost

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestHostsPersistFollowAddressesAndDoNotTransferToRecreatedVMs(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "outpost.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	vm, err := store.Create(ctx, "dev")
	if err != nil {
		t.Fatal(err)
	}
	host, err := store.SetHost(ctx, "Main-Dev.Example.Com.", "DEV", 3000)
	if err != nil {
		t.Fatal(err)
	}
	if host.Hostname != "main-dev.example.com" || host.OutpostID != vm.ID {
		t.Fatalf("host = %#v", host)
	}
	if _, err := store.SetHost(ctx, host.Hostname, "dev", 3000); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.SetState(ctx, vm.ID, DesiredRunning, StatusRunning, "172.30.0.8", ""); err != nil {
		t.Fatal(err)
	}
	resolved, err := store.Host(ctx, host.Hostname)
	if err != nil || resolved.GuestIP != "172.30.0.8" || resolved.Status != StatusRunning {
		t.Fatalf("resolved = %#v, %v", resolved, err)
	}
	if _, err := store.SetState(ctx, vm.ID, DesiredStopped, StatusStopped, "172.30.0.8", ""); err != nil {
		t.Fatal(err)
	}
	resolved, err = store.Host(ctx, host.Hostname)
	if err != nil || resolved.Status != StatusStopped {
		t.Fatalf("stopped = %#v, %v", resolved, err)
	}
	if _, err := store.SetState(ctx, vm.ID, DesiredRunning, StatusRunning, "172.30.0.9", ""); err != nil {
		t.Fatal(err)
	}
	resolved, err = store.Host(ctx, host.Hostname)
	if err != nil || resolved.GuestIP != "172.30.0.9" {
		t.Fatalf("changed address = %#v, %v", resolved, err)
	}
	if _, err := store.DeleteByID(ctx, vm.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Host(ctx, host.Hostname); !errors.Is(err, ErrHostNotFound) {
		t.Fatalf("deleted host error = %v", err)
	}
	// No orphaned hostname prevents a deliberate new registration.
	if _, err := store.SetHost(ctx, host.Hostname, "dev", 3000); err != nil {
		t.Fatal(err)
	}
	hosts, err := store.Hosts(ctx)
	if err != nil || len(hosts) != 1 || hosts[0].OutpostID == vm.ID {
		t.Fatalf("hosts = %#v, %v", hosts, err)
	}
}

func TestHostConflictsAndConcurrentRegistrationDoNotOverwrite(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "outpost.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, name := range []string{"one", "two"} {
		if _, err := store.Create(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.SetHost(ctx, "app.example.com", "one", 3000); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct {
		name string
		port int
	}{{"one", 3101}, {"two", 3000}} {
		if _, err := store.SetHost(ctx, "APP.example.com", input.name, input.port); !errors.Is(err, ErrHostTaken) {
			t.Fatalf("conflict = %v", err)
		}
	}
	var wg sync.WaitGroup
	failures := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.SetHost(ctx, "race.example.com", "one", 3000)
			failures <- err
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	hosts, err := store.Hosts(ctx)
	if err != nil || len(hosts) != 2 {
		t.Fatalf("hosts = %#v, %v", hosts, err)
	}
	if err := store.Unhost(ctx, "APP.EXAMPLE.COM."); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "one"); err != nil {
		t.Fatalf("unhost deleted VM: %v", err)
	}
}

func TestHostDeletionCascadeSurvivesConnectionReplacement(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "outpost.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	vm, err := store.Create(ctx, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetHost(ctx, "app.example.com", "dev", 3000); err != nil {
		t.Fatal(err)
	}
	store.db.SetMaxIdleConns(0)
	if _, err := store.DeleteByID(ctx, vm.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM hosts").Scan(&count); err != nil || count != 0 {
		t.Fatalf("orphaned routes after connection replacement: %d, %v", count, err)
	}
}

func TestHostRegistrySupportsMemoryDatabase(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Create(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetHost(ctx, "app.example.com", "dev", 3000); err != nil {
		t.Fatal(err)
	}
}

func TestHostValidation(t *testing.T) {
	for _, hostname := range []string{"", "localhost", "*.example.com", "https://example.com", "app.example.com:3000", "127.0.0.1", "bad_.example.com", "-bad.example.com", "a..example.com", strings.Repeat("a", 64) + ".example.com"} {
		if _, err := CanonicalHostname(hostname); !errors.Is(err, ErrInvalidHostname) {
			t.Errorf("hostname %q: %v", hostname, err)
		}
	}
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "outpost.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Create(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	for _, port := range []int{0, -1, 65536} {
		if _, err := store.SetHost(ctx, "app.example.com", "dev", port); !errors.Is(err, ErrInvalidPort) {
			t.Errorf("port %d: %v", port, err)
		}
	}
	if _, err := store.SetHost(ctx, "app.example.com", "missing", 3000); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing VM = %v", err)
	}
	hosts, err := store.Hosts(ctx)
	if err != nil || len(hosts) != 0 {
		t.Fatalf("invalid registrations persisted: %#v %v", hosts, err)
	}
}
