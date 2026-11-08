// Command testforge is the TestForge CLI and MCP server.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"

	"testforge/internal/cli"
	"testforge/internal/client"
	"testforge/internal/mcpserver"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	env := &cli.Env{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Getenv: os.Getenv,
		ServeMCP: func(ctx context.Context, c *client.Client, ws func(context.Context) (string, error)) error {
			return mcpserver.Serve(ctx, c, ws, cli.Version)
		},
	}
	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
		env.ReadSecret = func() (string, error) {
			b, err := term.ReadPassword(fd)
			return string(b), err
		}
	}
	code := cli.Main(ctx, os.Args[1:], env)
	stop()
	os.Exit(code)
}
