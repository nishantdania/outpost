package daemon

import "flag"

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
	flags.StringVar(
		&config.ListenAddr,
		"listen",
		":17890",
		"HTTP listen address",
	)
	flags.StringVar(
		&config.DatabasePath,
		"database",
		"./ark.db",
		"SQLite database path",
	)

	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}

	return config, nil
}
