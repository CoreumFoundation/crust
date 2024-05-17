package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/run"
	"github.com/CoreumFoundation/crust/build/types"
)

const maxStack = 100

// NewExecutor returns new executor.
func NewExecutor(commands map[string]types.Command) Executor {
	return Executor{commands: commands}
}

// Executor executes commands.
type Executor struct {
	commands map[string]types.Command
}

// Paths returns configured paths.
func (e Executor) Paths() []string {
	paths := make([]string, 0, len(e.commands))
	for path := range e.commands {
		paths = append(paths, path)
	}
	return paths
}

// Execute executes command specified by paths.
func (e Executor) Execute(ctx context.Context, paths []string) error {
	executed := map[reflect.Value]bool{}
	stack := map[reflect.Value]bool{}

	var depsFunc types.DepsFunc
	errReturn := errors.New("return")
	errChan := make(chan error, 1)
	worker := func(queue <-chan types.CommandFunc, done chan<- struct{}) {
		defer close(done)
		defer func() {
			if r := recover(); r != nil {
				var err error
				if err2, ok := r.(error); ok {
					//nolint:errorlint // comparison is ok
					if err2 == errReturn {
						return
					}
					err = err2
				} else {
					err = errors.Errorf("command panicked: %v", r)
				}
				errChan <- err
				close(errChan)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				close(errChan)
				return
			case cmd, ok := <-queue:
				if !ok {
					return
				}
				cmdValue := reflect.ValueOf(cmd)
				if executed[cmdValue] {
					continue
				}
				var err error
				switch {
				case stack[cmdValue]:
					err = errors.New("build: dependency cycle detected")
				case len(stack) >= maxStack:
					err = errors.New("build: maximum length of stack reached")
				default:
					stack[cmdValue] = true
					err = cmd(ctx, depsFunc)
					delete(stack, cmdValue)
					executed[cmdValue] = true
				}
				if err != nil {
					errChan <- err
					close(errChan)
					return
				}
			}
		}
	}
	depsFunc = func(deps ...types.CommandFunc) {
		queue := make(chan types.CommandFunc)
		done := make(chan struct{})
		go worker(queue, done)
	loop:
		for _, d := range deps {
			select {
			case <-done:
				break loop
			case queue <- d:
			}
		}
		close(queue)
		<-done
		if len(errChan) > 0 {
			panic(errReturn)
		}
	}

	initDeps := make([]types.CommandFunc, 0, len(paths))
	for _, p := range paths {
		if _, exists := e.commands[p]; !exists {
			return errors.Errorf("build: command %s does not exist", p)
		}
		initDeps = append(initDeps, e.commands[p].Fn)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				//nolint:errorlint // comparison is ok
				if err, ok := r.(error); ok && err == errReturn {
					return
				}
				panic(r)
			}
		}()
		depsFunc(initDeps...)
	}()
	if len(errChan) > 0 {
		return <-errChan
	}
	return nil
}

// Main receives configuration and runs commands.
func Main(commands map[string]types.Command) {
	run.Tool("build", func(ctx context.Context) error {
		var help bool

		flags := logger.Flags(logger.ToolDefaultConfig, "build")
		flags.BoolVarP(&help, "help", "h", false, "")
		if err := flags.Parse(os.Args[1:]); err != nil {
			return err
		}

		if help {
			listCommands(commands)
			return nil
		}

		executor := NewExecutor(commands)
		if isAutocomplete() {
			autocompleteDo(commands)
			return nil
		}

		changeWorkingDir()
		if len(flags.Args()) == 0 {
			return errors.New("no commands to execute provided")
		}
		return execute(ctx, flags.Args(), executor)
	})
}

func isAutocomplete() bool {
	_, ok := autocompletePrefix()
	return ok
}

func listCommands(commands map[string]types.Command) {
	paths := paths(commands)
	var maxLen int
	for _, path := range paths {
		if len(path) > maxLen {
			maxLen = len(path)
		}
	}
	fmt.Println("\n Available commands:")
	for _, path := range paths {
		fmt.Printf(fmt.Sprintf(`   %%-%ds`, maxLen)+"  %s\n", path, commands[path].Description)
	}
}

func execute(ctx context.Context, paths []string, executor Executor) error {
	pathsTrimmed := make([]string, 0, len(paths))
	for _, p := range paths {
		if p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
		pathsTrimmed = append(pathsTrimmed, p)
	}
	return executor.Execute(ctx, pathsTrimmed)
}

func autocompletePrefix() (string, bool) {
	exeName := os.Args[0]
	cLine := os.Getenv("COMP_LINE")
	cPoint := os.Getenv("COMP_POINT")

	if cLine == "" || cPoint == "" {
		return "", false
	}

	cPointInt, err := strconv.ParseInt(cPoint, 10, 64)
	if err != nil {
		panic(err)
	}

	prefix := strings.TrimLeft(cLine[:cPointInt], exeName)
	lastSpace := strings.LastIndex(prefix, " ") + 1
	return prefix[lastSpace:], true
}

func autocompleteDo(commands map[string]types.Command) {
	prefix, _ := autocompletePrefix()
	choices := choicesForPrefix(paths(commands), prefix)
	switch os.Getenv("COMP_TYPE") {
	case "9":
		startPos := strings.LastIndex(prefix, "/") + 1
		prefix = prefix[:startPos]
		if len(choices) == 1 {
			for choice, children := range choices {
				if children {
					choice += "/"
				} else {
					choice += " "
				}
				fmt.Println(prefix + choice)
			}
		} else if chPrefix := longestPrefix(choices); chPrefix != "" {
			fmt.Println(prefix + chPrefix)
		}
	case "63":
		if len(choices) > 1 {
			for choice, children := range choices {
				if children {
					choice += "/"
				}
				fmt.Println(choice)
			}
		}
	}
}

func paths(commands map[string]types.Command) []string {
	paths := make([]string, 0, len(commands))
	for path := range commands {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func choicesForPrefix(paths []string, prefix string) map[string]bool {
	startPos := strings.LastIndex(prefix, "/") + 1
	choices := map[string]bool{}
	for _, path := range paths {
		if strings.HasPrefix(path, prefix) {
			choice := path[startPos:]
			endPos := strings.Index(choice, "/")
			children := false
			if endPos != -1 {
				choice = choice[:endPos]
				children = true
			}
			if _, ok := choices[choice]; !ok || children {
				choices[choice] = children
			}
		}
	}
	return choices
}

func longestPrefix(choices map[string]bool) string {
	if len(choices) == 0 {
		return ""
	}
	prefix := ""
	for i := 0; true; i++ {
		var ch uint8
		for choice := range choices {
			if i >= len(choice) {
				return prefix
			}
			if ch == 0 {
				ch = choice[i]
				continue
			}
			if choice[i] != ch {
				return prefix
			}
		}
		prefix += string(ch)
	}
	return prefix
}

func changeWorkingDir() {
	must.OK(os.Chdir(filepath.Dir(filepath.Dir(filepath.Dir(must.String(
		filepath.EvalSymlinks(must.String(os.Executable()))))))))
}
