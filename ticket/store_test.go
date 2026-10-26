package ticket

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestOfflineTwoCloneConvergence(t *testing.T) {
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	gitTest(t, base, "init", "--bare", "-b", "main", remote)
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	gitTest(t, base, "clone", remote, a)
	gitTest(t, a, "config", "user.name", "Alice")
	gitTest(t, a, "config", "user.email", "alice@example.com")
	if err := os.WriteFile(filepath.Join(a, "README"), []byte("repo"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, a, "add", "README")
	gitTest(t, a, "-c", "commit.gpgsign=false", "commit", "-m", "init")
	gitTest(t, a, "push", "origin", "main")
	gitTest(t, base, "clone", remote, b)
	gitTest(t, b, "config", "user.name", "Bob")
	gitTest(t, b, "config", "user.email", "bob@example.com")
	sa, sb := Store{Repo: a}, Store{Repo: b}
	created, err := sa.Create("Fix preflight", "Detect capability", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := sa.Sync("push", "origin"); err != nil {
		t.Fatal(err)
	}
	if err := sb.Sync("pull", "origin"); err != nil {
		t.Fatal(err)
	}
	if _, err := sa.Comment(created.ID, "Alice saw the failure", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := sb.Comment(created.ID, "Bob reproduced offline", ""); err != nil {
		t.Fatal(err)
	}
	if err := sa.Sync("push", "origin"); err != nil {
		t.Fatal(err)
	}
	if err := sb.Sync("push", "origin"); err != nil {
		t.Fatal(err)
	}
	if err := sa.Sync("pull", "origin"); err != nil {
		t.Fatal(err)
	}
	if err := sb.Sync("pull", "origin"); err != nil {
		t.Fatal(err)
	}
	ta, err := sa.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	tb, err := sb.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ta.Events) != 3 || len(tb.Events) != 3 || ta.Events[1].ID != tb.Events[1].ID || ta.Events[2].ID != tb.Events[2].ID {
		t.Fatalf("replicas diverged: %#v vs %#v", ta, tb)
	}
}

func TestRejectInvalidUpdates(t *testing.T) {
	repo := t.TempDir()
	gitTest(t, repo, "init", "-b", "main")
	gitTest(t, repo, "config", "user.name", "Test")
	s := Store{Repo: repo}
	if _, err := s.Create("", "", ""); err == nil {
		t.Fatal("accepted blank title")
	}
	if _, err := s.Get("../../outside"); err == nil {
		t.Fatal("accepted invalid ticket ID")
	}
	ticket, err := s.Create("Task", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetStatus(ticket.ID, "unknown", ""); err == nil {
		t.Fatal("accepted invalid status")
	}
	claimed, err := s.Claim(ticket.ID, "")
	if err != nil || claimed.Assignee != "Test" || claimed.Status != "in_progress" {
		t.Fatalf("git identity was not used for claim: %#v, %v", claimed, err)
	}
}

func gitTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
