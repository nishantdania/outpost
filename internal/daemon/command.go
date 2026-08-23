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
	flags := flag.NewFlagSet("arkd", flag.ContinueOnError)
	config := Config{}
	flags.StringVar(&config.ListenAddr, "listen", "127.0.0.1:17890", "HTTP listen address")
	flags.StringVar(&config.DatabasePath, "database", "./ark.db", "SQLite database path")
	flags.StringVar(&config.Token, "token", os.Getenv("ARKD_TOKEN"), "bearer token (or ARKD_TOKEN)")
	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	return config, nil
}
