package exec

import (
	"os/exec"
)

// Docker runs docker command.
func Docker(args ...string) *exec.Cmd {
	return toolCmd("docker", args)
}

// Go runs go command.
func Go(args ...string) *exec.Cmd {
	return toolCmd("go", args)
}
