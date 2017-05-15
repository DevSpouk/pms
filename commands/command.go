// Package commands contains all functionality that is triggered by the user,
// either through keyboard bindings or the command-line interface. New commands
// such as 'sort', 'add', etc. must be implemented here.
package commands

import (
	"sort"

	"github.com/ambientsound/pms/api"
	"github.com/ambientsound/pms/input/lexer"
	"github.com/ambientsound/pms/parser"
)

// Verbs contain mappings from strings to Command constructors.
// Make sure to add commands here when implementing them, or they will not be recognized.
var Verbs = map[string]func(api.API) Command{
	"add":       NewAdd,
	"bind":      NewBind,
	"cursor":    NewCursor,
	"inputmode": NewInputMode,
	"isolate":   NewIsolate,
	"list":      NewList,
	"next":      NewNext,
	"pause":     NewPause,
	"play":      NewPlay,
	"prev":      NewPrevious,
	"previous":  NewPrevious,
	"print":     NewPrint,
	"q":         NewQuit,
	"quit":      NewQuit,
	"redraw":    NewRedraw,
	"remove":    NewRemove,
	"se":        NewSet,
	"select":    NewSelect,
	"set":       NewSet,
	"sort":      NewSort,
	"stop":      NewStop,
	"style":     NewStyle,
	"volume":    NewVolume,
}

// Command must be implemented by all commands.
type Command interface {
	// Execute parses the next input token.
	// FIXME: Execute is deprecated
	Execute(class int, s string) error

	// Exec executes the AST generated by the command.
	Exec() error

	// SetScanner assigns a scanner to the command.
	// FIXME: move to constructor?
	SetScanner(*lexer.Scanner)

	// Parse and make an abstract syntax tree. This function MUST NOT have any side effects.
	Parse() error

	// TabComplete returns a set of tokens that could possibly be used as the next
	// command parameter.
	TabComplete() []string

	// Scanned returns a slice of tokens that have been scanned using Parse().
	Scanned() []parser.Token
}

// command is a helper base class that all commands may use.
type command struct {
	cmdline string
}

// newcommand is an abolition which implements workarounds so that not
// everything in commands/ has to be refactored right away.
// FIXME
type newcommand struct {
	parser.Parser
	cmdline     string
	tabComplete []string
}

// New returns the Command associated with the given verb.
func New(verb string, a api.API) Command {
	ctor := Verbs[verb]
	if ctor == nil {
		return nil
	}
	return ctor(a)
}

// Keys returns a string slice with all verbs that can be invoked to run a command.
func Keys() []string {
	keys := make(sort.StringSlice, 0, len(Verbs))
	for verb := range Verbs {
		keys = append(keys, verb)
	}
	keys.Sort()
	return keys
}

// setTabComplete defines a string slice that will be used for tab completion
// at the current point in parsing.
func (c *newcommand) setTabComplete(s []string) {
	c.tabComplete = s
}

// setTabCompleteEmpty removes all tab completions.
func (c *newcommand) setTabCompleteEmpty() {
	c.setTabComplete([]string{})
}

// TabComplete implements Command.TabComplete.
func (c *newcommand) TabComplete() []string {
	if c.tabComplete == nil {
		// FIXME
		return make([]string, 0)
	}
	return c.tabComplete
}

// Execute implements Command.Execute.
// FIXME: boilerplate until Execute is removed from interface
func (c *newcommand) Execute(class int, s string) error {
	return nil
}

//
// These functions belong to the old implementation.
// FIXME: remove everything below.
//

// Parse implements Command.Parse.
func (c *command) SetScanner(s *lexer.Scanner) {
}

// Parse implements Command.Parse.
func (c *command) Parse() error {
	return nil
}

// Scanned implements Command.Scanned.
func (c *command) Scanned() []parser.Token {
	return make([]parser.Token, 0)
}

// TabComplete implements Command.TabComplete.
func (c *command) TabComplete() []string {
	return []string{}
}

// Exec implements Command.TabComplete.
func (c *command) Exec() error {
	return nil
}
