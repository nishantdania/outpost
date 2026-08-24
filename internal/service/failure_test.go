package service

import (
	"context"
	"errors"
	"testing"

	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/testutil"
)

func TestManagerFailuresAreRetained(t *testing.T) {
	failure := errors.New("manager failed")
	for _, test := range []struct {
		name    string
		manager *testutil.FakeManager
		action  func(*Service) error
		desired string
	}{
		{"create", &testutil.FakeManager{CreateFunc: func(context.Context, outpost.Outpost) error { return failure }}, func(s *Service) error { _, err := s.Create(context.Background(), input("demo")); return err }, outpost.DesiredRunning},
		{"start", &testutil.FakeManager{StartFunc: func(context.Context, string) (string, error) { return "", failure }}, func(s *Service) error { _, err := s.Create(context.Background(), input("demo")); return err }, outpost.DesiredRunning},
		{"stop", &testutil.FakeManager{StopFunc: func(context.Context, string) error { return failure }}, func(s *Service) error {
			if _, err := s.Create(context.Background(), input("demo")); err != nil {
				return err
			}
			_, err := s.Stop(context.Background(), "demo")
			return err
		}, outpost.DesiredStopped},
		{"delete", &testutil.FakeManager{DeleteFunc: func(context.Context, string) error { return failure }}, func(s *Service) error {
			if _, err := s.Create(context.Background(), input("demo")); err != nil {
				return err
			}
			_, err := s.Delete(context.Background(), "demo")
			return err
		}, outpost.DesiredDeleted},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := testStore(t)
			application := New(store, test.manager)
			if err := test.action(application); !errors.Is(err, failure) {
				t.Fatalf("action error = %v, want %v", err, failure)
			}
			a, err := application.Get(context.Background(), "demo")
			if err != nil {
				t.Fatal(err)
			}
			if a.Status != outpost.StatusFailed || a.DesiredState != test.desired || a.Failure != failure.Error() {
				t.Fatalf("failed Outpost = %#v", a)
			}
		})
	}
}
