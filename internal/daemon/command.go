package daemon

import (
	"flag"
	"os"
)

func Execute(args []string) error {
	config, err := parseConfig(args)
	if err != nil {
		return err
	}
	return Run(config)
}
func parseConfig(args []string) (Config, error) {
	flags := flag.NewFlagSet("outpostd", flag.ContinueOnError)
	config := Config{}
	flags.StringVar(&config.ListenAddr, "listen", "127.0.0.1:17890", "HTTP listen address")
	flags.StringVar(&config.DatabasePath, "database", "./outpost.db", "SQLite database path")
	flags.StringVar(&config.Token, "token", os.Getenv("OUTPOSTD_TOKEN"), "bearer token (or OUTPOSTD_TOKEN)")
	flags.StringVar(&config.LauncherSocket, "launcher-socket", "/run/outpost/vm-launcher.sock", "VM launcher Unix socket")
	flags.StringVar(&config.ImageStore, "image-store", "/var/lib/outpostd/images", "custom image store")
	flags.StringVar(&config.DefaultOCI, "default-oci", "/usr/local/lib/outpost/default.oci.tar", "default OCI archive")
	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	return config, nil
}
