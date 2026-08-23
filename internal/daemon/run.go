package daemon

import "github.com/nishantdania/ark/internal/httpapi"

func Run(config Config) error {
	server := httpapi.NewServer(config.ListenAddr)

	return server.ListenAndServe()
}
