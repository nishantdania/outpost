package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

const helpText = `Usage:
  outpost [--host <name>] <command>

Commands:
  create    Create an Outpost
  list      List Outposts (alias: ls)
  start     Start an Outpost
  stop      Stop an Outpost
  ssh       Connect to an Outpost
  exec      Run a bash command in an Outpost
  copy      Copy a file into an Outpost (alias: cp)
  delete    Delete an Outpost
  setup     Prepare a server for Firecracker
  doctor    Check server readiness
  update    Update Outpost
  uninstall Remove Outpost
  version   Show versions
  help      Show help
`

func Run(ctx context.Context, args []string, version string, stdout, stderr io.Writer) int {
	parsed, restore, err := selectHost(args)
	if err != nil {
		return run(err, "host", stderr)
	}
	defer restore()
	args = parsed
	if helpRequested(args) {
		return printHelp(args, stdout)
	}
	switch {
	case len(args) == 0 || isHelp(args):
		fmt.Fprint(stdout, helpText)
		return 0
	case len(args) >= 1 && args[0] == "create":
		options, err := parseCreateArgs(args[1:])
		if err != nil {
			return run(err, "create", stderr)
		}
		return run(create(ctx, options, stdout), "create", stderr)
	case len(args) == 1 && (args[0] == "list" || args[0] == "ls"):
		return run(list(ctx, stdout), "list", stderr)
	case len(args) == 2 && args[0] == "start":
		return run(lifecycle(ctx, args[1], "start", stdout), "start", stderr)
	case len(args) == 2 && args[0] == "stop":
		return run(lifecycle(ctx, args[1], "stop", stdout), "stop", stderr)
	case len(args) == 2 && args[0] == "ssh":
		return run(sshOutpost(ctx, args[1]), "ssh", stderr)
	case len(args) >= 4 && args[0] == "exec" && (args[1] == "-i" || args[1] == "--interactive"):
		return runExec(execOutpost(ctx, args[2], strings.Join(args[3:], " "), true), stderr)
	case len(args) >= 3 && args[0] == "exec":
		return runExec(execOutpost(ctx, args[1], strings.Join(args[2:], " "), false), stderr)
	case len(args) >= 3 && (args[0] == "copy" || args[0] == "cp"):
		source, target, mode, err := copyArgs(args[1:])
		if err != nil {
			return run(err, "copy", stderr)
		}
		return run(copyToOutpost(ctx, source, target, mode, stdout), "copy", stderr)
	case len(args) == 2 && args[0] == "delete":
		return run(deleteOutpost(ctx, args[1], stdout), "delete", stderr)
	case len(args) == 1 && args[0] == "setup":
		return run(setup(ctx), "setup", stderr)
	case len(args) == 1 && args[0] == "doctor":
		return run(doctor(ctx, stdout), "doctor", stderr)
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
func selectHost(args []string) ([]string, func(), error) {
	if len(args) == 0 || (args[0] != "--host" && !strings.HasPrefix(args[0], "--host=")) {
		return args, func() {}, nil
	}
	name := strings.TrimPrefix(args[0], "--host=")
	consumed := 1
	if args[0] == "--host" {
		if len(args) < 2 {
			return nil, func() {}, fmt.Errorf("--host requires a name")
		}
		name, consumed = args[1], 2
	}
	if name == "" {
		return nil, func() {}, fmt.Errorf("--host requires a name")
	}
	previous, existed := os.LookupEnv("OUTPOST_HOST")
	if err := os.Setenv("OUTPOST_HOST", name); err != nil {
		return nil, func() {}, err
	}
	restore := func() {
		if existed {
			_ = os.Setenv("OUTPOST_HOST", previous)
		} else {
			_ = os.Unsetenv("OUTPOST_HOST")
		}
	}
	return args[consumed:], restore, nil
}

func isHelp(args []string) bool {
	return len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help")
}

func helpRequested(args []string) bool {
	if len(args) < 2 {
		return false
	}
	for _, arg := range args[1:] {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func printHelp(args []string, stdout io.Writer) int {
	command := args[0]
	if command == "cp" {
		command = "copy"
	}
	if command == "ls" {
		command = "list"
	}
	type help struct {
		usage       string
		description string
		examples    []string
	}
	commands := map[string]help{
		"create":    {"outpost create [name] [--cpus N] [--memory SIZE] [--disk SIZE]", "Create and start a new Outpost. Defaults: 2 vCPU, 4 GiB RAM, 8 GiB disk.", []string{"outpost create dev", "outpost create build --cpus 4 --memory 8G --disk 32G"}},
		"list":      {"outpost list", "List Outposts and their runtime status. Alias: ls.", []string{"outpost list", "outpost ls"}},
		"start":     {"outpost start <id>", "Start a stopped Outpost.", []string{"outpost start <id>"}},
		"stop":      {"outpost stop <id>", "Stop an Outpost without deleting its disk.", []string{"outpost stop <id>"}},
		"ssh":       {"outpost ssh <id|name>", "Open an interactive SSH session to an Outpost.", []string{"outpost ssh dev"}},
		"exec":      {"outpost exec [-i] <id|name> <command>", "Run a Bash command in an Outpost; -i forwards stdin.", []string{"outpost exec dev 'uname -a'", "printf 'hello' | outpost exec -i dev 'cat'"}},
		"copy":      {"outpost copy [--mode MODE] <source> <id|name>:<destination>", "Copy a local, stdin, or host file into an Outpost. Alias: cp.", []string{"outpost copy ./file dev:/root/file", "outpost copy --mode 600 host:~/.config/app dev:/root/.config/app"}},
		"delete":    {"outpost delete <id|name>", "Stop and permanently delete an Outpost.", []string{"outpost delete dev"}},
		"setup":     {"outpost setup", "Prepare the server, VM assets, networking, and SSH access.", nil},
		"doctor":    {"outpost doctor", "Check whether the server is ready to run Outposts.", nil},
		"update":    {"outpost update [local|server]", "Update the local CLI, server daemon, or both.", []string{"outpost update", "outpost update server"}},
		"uninstall": {"outpost uninstall [local|server]", "Remove Outpost and its managed resources.", []string{"outpost uninstall server"}},
		"version":   {"outpost version [local|server]", "Show local and server Outpost versions.", []string{"outpost version", "outpost version server"}},
	}
	if value, ok := commands[command]; ok {
		fmt.Fprintf(stdout, "Usage: %s\n\n%s\n", value.usage, value.description)
		if len(value.examples) > 0 {
			fmt.Fprintln(stdout, "\nExamples:")
			for _, example := range value.examples {
				fmt.Fprintf(stdout, "  %s\n", example)
			}
		}
	} else {
		fmt.Fprintf(stdout, "No help available for %s.\n", args[0])
	}
	return 0
}

func run(err error, command string, stderr io.Writer) int {
	if err != nil {
		fmt.Fprintf(stderr, "outpost %s: %v\n", command, err)
		return 1
	}
	return 0
}
