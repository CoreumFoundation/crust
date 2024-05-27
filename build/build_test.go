package build

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/CoreumFoundation/crust/build/types"
)

type report map[int]string

func cmdA(r report, cmdAA, cmdAB types.CommandFunc) types.CommandFunc {
	return func(ctx context.Context, deps types.DepsFunc) error {
		deps(cmdAA, cmdAB)
		r[len(r)] = "a"
		return nil
	}
}

func cmdAA(r report, cmdAC types.CommandFunc) types.CommandFunc {
	return func(ctx context.Context, deps types.DepsFunc) error {
		deps(cmdAC)
		r[len(r)] = "aa"
		return nil
	}
}

func cmdAB(r report, cmdAC types.CommandFunc) types.CommandFunc {
	return func(ctx context.Context, deps types.DepsFunc) error {
		deps(cmdAC)
		r[len(r)] = "ab"
		return nil
	}
}

func cmdAC(r report) types.CommandFunc {
	return func(ctx context.Context, deps types.DepsFunc) error {
		r[len(r)] = "ac"
		return nil
	}
}

func cmdB(ctx context.Context, deps types.DepsFunc) error {
	return errors.New("error")
}

func cmdC(ctx context.Context, deps types.DepsFunc) error {
	deps(cmdD)
	return nil
}

func cmdD(ctx context.Context, deps types.DepsFunc) error {
	deps(cmdC)
	return nil
}

func cmdE(ctx context.Context, deps types.DepsFunc) error {
	panic("panic")
}

func cmdF(ctx context.Context, deps types.DepsFunc) error {
	<-ctx.Done()
	return ctx.Err()
}

var tCtx = context.Background()

func setup() (Executor, report) {
	r := report{}

	cmdAC := cmdAC(r)
	cmdAA := cmdAA(r, cmdAC)
	cmdAB := cmdAB(r, cmdAC)
	commands := map[string]types.Command{
		"a":    {Fn: cmdA(r, cmdAA, cmdAB)},
		"a/aa": {Fn: cmdAA},
		"a/ab": {Fn: cmdAB},
		"b":    {Fn: cmdB},
		"c":    {Fn: cmdC},
		"d":    {Fn: cmdD},
		"e":    {Fn: cmdE},
		"f":    {Fn: cmdF},
	}

	return NewExecutor(commands), r
}

func TestRootCommand(t *testing.T) {
	exe, r := setup()
	require.NoError(t, execute(tCtx, []string{"a"}, exe))

	assert.Len(t, r, 4)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
	assert.Equal(t, "ab", r[2])
	assert.Equal(t, "a", r[3])
}

func TestChildCommand(t *testing.T) {
	exe, r := setup()
	require.NoError(t, execute(tCtx, []string{"a/aa"}, exe))

	assert.Len(t, r, 2)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
}

func TestTwoCommands(t *testing.T) {
	exe, r := setup()
	require.NoError(t, execute(tCtx, []string{"a/aa", "a/ab"}, exe))

	assert.Len(t, r, 3)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
	assert.Equal(t, "ab", r[2])
}

func TestCommandWithSlash(t *testing.T) {
	exe, r := setup()
	require.NoError(t, execute(tCtx, []string{"a/aa/"}, exe))

	assert.Len(t, r, 2)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
}

func TestCommandsAreExecutedOnce(t *testing.T) {
	exe, r := setup()
	require.NoError(t, execute(tCtx, []string{"a", "a"}, exe))

	assert.Len(t, r, 4)
	assert.Equal(t, "ac", r[0])
	assert.Equal(t, "aa", r[1])
	assert.Equal(t, "ab", r[2])
	assert.Equal(t, "a", r[3])
}

func TestCommandReturnsError(t *testing.T) {
	exe, _ := setup()
	require.Error(t, execute(tCtx, []string{"b"}, exe))
}

func TestCommandPanics(t *testing.T) {
	exe, _ := setup()
	require.Error(t, execute(tCtx, []string{"e"}, exe))
}

func TestErrorOnCyclicDependencies(t *testing.T) {
	exe, _ := setup()
	require.Error(t, execute(tCtx, []string{"c"}, exe))
}

func TestRootCommandDoesNotExist(t *testing.T) {
	exe, _ := setup()
	require.Error(t, execute(tCtx, []string{"z"}, exe))
}

func TestChildCommandDoesNotExist(t *testing.T) {
	exe, _ := setup()
	require.Error(t, execute(tCtx, []string{"a/z"}, exe))
}

func TestCommandStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(tCtx)
	cancel()
	exe, _ := setup()
	err := execute(ctx, []string{"f"}, exe)
	assert.Equal(t, context.Canceled, err)
}
