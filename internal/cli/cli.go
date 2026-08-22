package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
)

const helpText = `Usage:
  outpost <command>

Commands:
  create    Create an Outpost
  list      List Outposts
  delete    Delete an Outpost
  setup     Prepare a server for Firecracker
  update    Update Outpost
  uninstall Remove Outpost
  version   Show versions
  help      Show help
`

func Run(ctx context.Context, args []string, version string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0 || isHelp(args):
		fmt.Fprint(stdout, helpText)
		return 0
	case (len(args) == 1 || len(args) == 2) && args[0] == "create":
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		return run(create(ctx, name, stdout), "create", stderr)
	case len(args) == 1 && args[0] == "list":
		return run(list(ctx, stdout), "list", stderr)
	case len(args) == 2 && args[0] == "delete":
		return run(deleteOutpost(ctx, args[1], stdout), "delete", stderr)
	case len(args) == 1 && args[0] == "setup":
		return run(setup(ctx), "setup", stderr)
	case len(args) == 1 && args[0] == "version":
		return run(versions(ctx, version, stdout), "version", stderr)
	case len(args) == 2 && args[0] == "version" && args[1] == "local":
		fmt.Fprintf(stdout, "outpost %s\n", version)
		return 0
	case len(args) == 2 && args[0] == "version" && args[1] == "server":
		return run(serverVersion(ctx, stdout), "version server", stderr)
	case len(args) == 1 && args[0] == "update":
		if code := run(serverUpdate(ctx, stdout), "update", stderr); code != 0 {
			return code
		}
		return run(localUpdate(ctx, version, stdout), "update", stderr)
	case len(args) == 2 && args[0] == "update" && args[1] == "local":
		return run(localUpdate(ctx, version, stdout), "update local", stderr)
	case len(args) == 2 && args[0] == "update" && args[1] == "server":
		return run(serverUpdate(ctx, stdout), "update server", stderr)
	case len(args) == 1 && args[0] == "uninstall":
		if code := run(serverUninstall(ctx, stdout), "uninstall", stderr); code != 0 {
			return code
		}
		return run(localUninstall(stdout), "uninstall", stderr)
	case len(args) == 2 && args[0] == "uninstall" && args[1] == "local":
		return run(localUninstall(stdout), "uninstall local", stderr)
	case len(args) == 2 && args[0] == "uninstall" && args[1] == "server":
		return run(serverUninstall(ctx, stdout), "uninstall server", stderr)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", strings.Join(args, " "))
		fmt.Fprint(stderr, helpText)
		return 2
	}
}
func isHelp(args []string) bool {
	return len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help")
}
func run(err error, command string, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "outpost %s: %v\n", command, err)
		return 1
	}
	return 0
}
