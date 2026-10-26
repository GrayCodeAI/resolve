package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/GrayCodeAI/resolve/ticket"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "resolve:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("use new, list, show, comment, claim, status, or sync")
	}
	name := args[0]
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	repo := fs.String("repo", ".", "Git repository")
	actor := fs.String("actor", "", "actor name; defaults to git user.name")
	title := fs.String("title", "", "ticket title")
	body := fs.String("body", "", "ticket description")
	text := fs.String("text", "", "comment text")
	status := fs.String("set", "", "new ticket status")
	remote := fs.String("remote", "origin", "Git remote")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	s := ticket.Store{Repo: *repo}
	switch name {
	case "new":
		v, err := s.Create(*title, *body, *actor)
		return output(v, err)
	case "list":
		v, err := s.List()
		return output(v, err)
	case "show", "comment", "claim", "status":
		if fs.NArg() != 1 {
			return fmt.Errorf("%s requires a ticket ID", name)
		}
		id := fs.Arg(0)
		switch name {
		case "show":
			v, err := s.Get(id)
			return output(v, err)
		case "comment":
			v, err := s.Comment(id, *text, *actor)
			return output(v, err)
		case "claim":
			v, err := s.Claim(id, *actor)
			return output(v, err)
		default:
			v, err := s.SetStatus(id, *status, *actor)
			return output(v, err)
		}
	case "sync":
		if fs.NArg() != 1 {
			return errors.New("sync requires pull or push")
		}
		return s.Sync(fs.Arg(0), *remote)
	default:
		return fmt.Errorf("unknown command %q", name)
	}
}

func output(v any, err error) error {
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(v)
}
