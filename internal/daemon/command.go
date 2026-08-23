package daemon

import "flag"

func Execute(args []string) error {
	flags := flag.NewFlagSet("arkd", flag.ContinueOnError)

	config := Config{}
	flags.StringVar(
		&config.ListenAddr,
		"listen",
		":17890",
		"HTTP listen address",
	)

	if err := flags.Parse(args); err != nil {
		return err
	}

	return Run(config)
}
