package service

import (
	"context"
	"errors"
	"time"

	"github.com/nishantdania/outpost/internal/credentials"
	"github.com/nishantdania/outpost/internal/outpost"
)

var ErrCredentialsUnavailable = errors.New("credential egress is not configured on this server")

func (s *Service) WithCredentials(host *credentials.Host) *Service { s.credentials = host; return s }
func (s *Service) checkCredentials(config string) error {
	if config == "" {
		return nil
	}
	if s.credentials == nil {
		return ErrCredentialsUnavailable
	}
	p, err := credentials.Parse([]byte(config))
	if err != nil {
		return err
	}
	return s.credentials.Check(p)
}
func (s *Service) guestCredentials(a *outpost.Outpost) error {
	if err := s.checkCredentials(a.CredentialConfig); err != nil {
		return err
	}
	if a.CredentialConfig != "" {
		p, _ := credentials.Parse([]byte(a.CredentialConfig))
		a.CredentialEnv = p.Environment()
		a.EgressCA = s.credentials.PublicCA
	}
	return nil
}
func CredentialAuthorizer(store *outpost.Store) credentials.Authorize {
	return func(ctx context.Context, ip string) (credentials.Grant, error) {
		a, err := store.CredentialGuest(ctx, ip)
		if err != nil {
			return credentials.Grant{}, err
		}
		p, err := credentials.Parse([]byte(a.CredentialConfig))
		if err != nil {
			return credentials.Grant{}, err
		}
		return credentials.Grant{ID: a.ID, Epoch: a.UpdatedAt.Format(time.RFC3339Nano), Profile: p}, nil
	}
}
