package base

import (
	"fmt"
	"io"
	"sort"
	"sync"
)

type HandlerFunc func(args []string, stdout io.Writer) error

type Command interface {
	Name() string
	Help() string
	Run(args []string, stdout io.Writer) error
}

type command struct {
	name    string
	help    string
	handler HandlerFunc
}

func (c command) Name() string {
	return c.name
}

func (c command) Help() string {
	return c.help
}

func (c command) Run(args []string, stdout io.Writer) error {
	return c.handler(args, stdout)
}

func NewCommand(name, help string, handler HandlerFunc) Command {
	return command{
		name:    name,
		help:    help,
		handler: handler,
	}
}

func Adapt(handler func(args []string) error) HandlerFunc {
	return func(args []string, _ io.Writer) error {
		return handler(args)
	}
}

var registry = struct {
	sync.RWMutex
	byName map[string]Command
	order  []string
}{
	byName: make(map[string]Command),
}

func Register(cmd Command) {
	registry.Lock()
	defer registry.Unlock()

	name := cmd.Name()
	if _, exists := registry.byName[name]; exists {
		panic(fmt.Sprintf("command already registered: %s", name))
	}

	registry.byName[name] = cmd
	registry.order = append(registry.order, name)
}

func Lookup(name string) (Command, bool) {
	registry.RLock()
	defer registry.RUnlock()

	cmd, ok := registry.byName[name]
	return cmd, ok
}

func Commands() []Command {
	registry.RLock()
	defer registry.RUnlock()

	names := append([]string(nil), registry.order...)
	sort.Strings(names)

	commands := make([]Command, 0, len(names))
	for _, name := range names {
		commands = append(commands, registry.byName[name])
	}
	return commands
}
