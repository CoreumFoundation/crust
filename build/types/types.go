package types

import "context"

// CommandFunc represents executable command.
type CommandFunc func(ctx context.Context, deps DepsFunc) error

// DepsFunc represents function for executing dependencies.
type DepsFunc func(deps ...CommandFunc)

// Command defines the command.
type Command struct {
	Description string
	Fn          CommandFunc
}
