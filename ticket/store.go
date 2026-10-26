package ticket

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const refPrefix = "refs/resolve/events/"

type Event struct {
	Version  int       `json:"version"`
	ID       string    `json:"id"`
	TicketID string    `json:"ticket_id"`
	Kind     string    `json:"kind"`
	Actor    string    `json:"actor"`
	Clock    uint64    `json:"clock"`
	Time     time.Time `json:"time"`
	Title    string    `json:"title,omitempty"`
	Body     string    `json:"body,omitempty"`
	Status   string    `json:"status,omitempty"`
	Text     string    `json:"text,omitempty"`
}

type Ticket struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Body     string  `json:"body"`
	Status   string  `json:"status"`
	Assignee string  `json:"assignee,omitempty"`
	Events   []Event `json:"events"`
}

type Store struct{ Repo string }

func (s Store) Create(title, body, actor string) (Ticket, error) {
	if strings.TrimSpace(title) == "" {
		return Ticket{}, errors.New("title is required")
	}
	id, err := randomID()
	if err != nil {
		return Ticket{}, err
	}
	e := Event{Version: 1, TicketID: id, Kind: "created", Actor: actor, Clock: 1, Time: time.Now().UTC(), Title: strings.TrimSpace(title), Body: strings.TrimSpace(body), Status: "open"}
	if err := s.writeEvent(&e); err != nil {
		return Ticket{}, err
	}
	return s.Get(id)
}

func (s Store) Comment(id, text, actor string) (Ticket, error) {
	if strings.TrimSpace(text) == "" {
		return Ticket{}, errors.New("comment is required")
	}
	return s.append(id, Event{Kind: "comment", Actor: actor, Text: strings.TrimSpace(text)})
}

func (s Store) SetStatus(id, status, actor string) (Ticket, error) {
	switch status {
	case "open", "in_progress", "in_review", "done":
	default:
		return Ticket{}, fmt.Errorf("invalid status %q", status)
	}
	return s.append(id, Event{Kind: "status", Actor: actor, Status: status})
}

func (s Store) Claim(id, actor string) (Ticket, error) {
	return s.append(id, Event{Kind: "claim", Actor: actor})
}

func (s Store) append(id string, event Event) (Ticket, error) {
	t, err := s.Get(id)
	if err != nil {
		return Ticket{}, err
	}
	var maxClock uint64
	for _, e := range t.Events {
		if e.Clock > maxClock {
			maxClock = e.Clock
		}
	}
	event.Version, event.TicketID, event.Clock, event.Time = 1, id, maxClock+1, time.Now().UTC()
	if err := s.writeEvent(&event); err != nil {
		return Ticket{}, err
	}
	return s.Get(id)
}

func (s Store) Get(id string) (Ticket, error) {
	if !validID(id) {
		return Ticket{}, errors.New("invalid ticket ID")
	}
	events, err := s.events(id)
	if err != nil {
		return Ticket{}, err
	}
	if len(events) == 0 {
		return Ticket{}, errors.New("ticket not found")
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].Clock == events[j].Clock {
			return events[i].ID < events[j].ID
		}
		return events[i].Clock < events[j].Clock
	})
	t := Ticket{ID: id, Events: events}
	created := false
	for _, e := range events {
		switch e.Kind {
		case "created":
			if created {
				return Ticket{}, errors.New("ticket has multiple creation events")
			}
			t.Title, t.Body, t.Status = e.Title, e.Body, "open"
			created = true
		case "status":
			t.Status = e.Status
		case "claim":
			t.Assignee, t.Status = e.Actor, "in_progress"
		}
	}
	if !created {
		return Ticket{}, errors.New("ticket is missing its creation event")
	}
	return t, nil
}

func (s Store) List() ([]Ticket, error) {
	events, err := s.events("")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Ticket
	for _, e := range events {
		if seen[e.TicketID] {
			continue
		}
		seen[e.TicketID] = true
		t, err := s.Get(e.TicketID)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s Store) Sync(direction, remote string) error {
	if remote == "" || strings.HasPrefix(remote, "-") {
		return errors.New("a configured Git remote name is required")
	}
	switch direction {
	case "pull":
		_, err := s.git(nil, "fetch", remote, refPrefix+"*:"+refPrefix+"*")
		return err
	case "push":
		_, err := s.git(nil, "push", remote, refPrefix+"*:"+refPrefix+"*")
		return err
	default:
		return errors.New("direction must be pull or push")
	}
}

func (s Store) events(id string) ([]Event, error) {
	prefix := refPrefix
	if id != "" {
		prefix += id + "/"
	}
	out, err := s.git(nil, "for-each-ref", "--format=%(refname) %(objectname)", prefix)
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 || !strings.HasPrefix(parts[0], prefix) {
			return nil, errors.New("invalid ticket reference")
		}
		data, err := s.git(nil, "cat-file", "blob", parts[1])
		if err != nil {
			return nil, err
		}
		var e Event
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, err
		}
		if err := validateEvent(e); err != nil || parts[0] != prefixFor(e) {
			return nil, errors.New("invalid or unsupported ticket event")
		}
		events = append(events, e)
	}
	return events, nil
}

func (s Store) writeEvent(e *Event) error {
	if e.Actor == "" {
		actor, err := s.git(nil, "config", "user.name")
		if err != nil || strings.TrimSpace(string(actor)) == "" {
			return errors.New("actor or git user.name is required")
		}
		e.Actor = strings.TrimSpace(string(actor))
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	e.ID = id
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	oid, err := s.git(data, "hash-object", "-w", "--stdin")
	if err != nil {
		return err
	}
	_, err = s.git(nil, "update-ref", prefixFor(*e), strings.TrimSpace(string(oid)), strings.Repeat("0", 40))
	return err
}

func prefixFor(e Event) string { return refPrefix + e.TicketID + "/" + e.ID }

func validateEvent(e Event) error {
	if e.Version != 1 || !validID(e.ID) || !validID(e.TicketID) || strings.TrimSpace(e.Actor) == "" || e.Clock == 0 {
		return errors.New("invalid event metadata")
	}
	switch e.Kind {
	case "created":
		if e.Clock != 1 || strings.TrimSpace(e.Title) == "" || e.Status != "open" {
			return errors.New("invalid creation event")
		}
	case "comment":
		if strings.TrimSpace(e.Text) == "" {
			return errors.New("empty comment")
		}
	case "claim":
	case "status":
		switch e.Status {
		case "open", "in_progress", "in_review", "done":
		default:
			return errors.New("invalid status event")
		}
	default:
		return errors.New("unknown event kind")
	}
	return nil
}

func (s Store) git(input []byte, args ...string) ([]byte, error) {
	repo := s.Repo
	if repo == "" {
		repo = "."
	}
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func randomID() (string, error) {
	var b [16]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}

func validID(id string) bool {
	if len(id) != 32 || id != strings.ToLower(id) {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
