package exec

import (
	"os/exec"
)

// Docker runs docker command.
func Docker(args ...string) *exec.Cmd {
	return toolCmd("docker", args)
}

func Go(args ...string) *exec.Cmd {
	return toolCmd("go", args)
}
