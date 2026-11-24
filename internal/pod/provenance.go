package pod

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// EngineDir is the pinned engine checkout, relative to the repo root.
const EngineDir = "third_party/gorge"

// Provenance pins what a run was played on. Nothing in it depends on the host.
type Provenance struct {
	EngineRepo  string `json:"engine_repo"`
	EngineRev   string `json:"engine_rev"`
	EngineDirty bool   `json:"engine_dirty"` // tracked files differ from EngineRev
	CardsRepo   string `json:"cards_repo"`
	CardsRev    string `json:"cards_rev"`
	CardsDigest string `json:"cards_digest"`
	TableKey    string `json:"table_key"` // dirac's key for this binary and corpus
}

// Host describes the machine a timing was taken on.
type Host struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	CPU    string `json:"cpu,omitempty"`
	NumCPU int    `json:"num_cpu"`
	Go     string `json:"go"`
}

// FindRoot walks up from the working directory to the repo root: the directory
// holding both decks/ and the engine checkout.
func FindRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isDir(filepath.Join(dir, "decks")) && isFile(filepath.Join(dir, EngineDir, "go.mod")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("repo root not found: run inside the checkout (did you run `make setup`?) or pass -root")
		}
		dir = parent
	}
}

// UseCorpus points dirac at the engine checkout's card corpus unless the
// caller already chose one through GORGE_CARDS or DIRAC_TABLES.
func UseCorpus(root string) error {
	if os.Getenv("GORGE_CARDS") != "" || os.Getenv("DIRAC_TABLES") != "" {
		return nil
	}
	cards := filepath.Join(root, EngineDir, ".cards")
	if !isFile(filepath.Join(cards, "cards.lock")) {
		return fmt.Errorf("no card corpus at %s: run `make setup`", cards)
	}
	return os.Setenv("GORGE_CARDS", cards)
}

// ReadProvenance collects the engine and card revisions for root.
func ReadProvenance(root string) (Provenance, error) {
	var p Provenance
	engine := filepath.Join(root, EngineDir)
	p.EngineRepo = git(engine, "config", "--get", "remote.origin.url")
	p.EngineRev = git(engine, "rev-parse", "HEAD")
	if p.EngineRev == "" {
		return p, fmt.Errorf("%s is not a git checkout: run `make setup`", EngineDir)
	}
	p.EngineDirty = git(engine, "status", "--porcelain", "--untracked-files=no") != ""

	corpus, err := cardset.CorpusDir()
	if err != nil {
		return p, err
	}
	raw, err := os.ReadFile(filepath.Join(corpus, "cards.lock"))
	if err != nil {
		return p, err
	}
	var lock struct {
		Repo   string `json:"repo"`
		Commit string `json:"commit"`
		Digest string `json:"digest"`
	}
	if err := json.Unmarshal(raw, &lock); err != nil {
		return p, fmt.Errorf("cards.lock: %w", err)
	}
	p.CardsRepo, p.CardsRev, p.CardsDigest = lock.Repo, lock.Commit, lock.Digest
	if p.TableKey, err = cardset.Key(); err != nil {
		return p, err
	}
	return p, nil
}

// ThisHost describes the current machine.
func ThisHost() Host {
	h := Host{OS: runtime.GOOS, Arch: runtime.GOARCH, NumCPU: runtime.NumCPU(), Go: runtime.Version()}
	switch runtime.GOOS {
	case "darwin":
		h.CPU = run("sysctl", "-n", "machdep.cpu.brand_string")
	case "linux":
		if raw, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				if k, v, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(k) == "model name" {
					h.CPU = strings.TrimSpace(v)
					break
				}
			}
		}
	}
	return h
}

func git(dir string, args ...string) string {
	return run("git", append([]string{"-C", dir}, args...)...)
}

func run(name string, args ...string) string {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
