package daemon

import "github.com/nishantdania/ark/internal/httpapi"

func Run() error {
	server := httpapi.NewServer(":17890")

	return server.ListenAndServe()
}
