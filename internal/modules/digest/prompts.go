package digest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	promptSummary = "summary.md"
	promptDigest  = "digest.md"
	promptCluster = "cluster.md"
	promptTopic   = "topic.md"
)

// prompts holds loaded prompt templates.
type prompts struct {
	Summary string
	Digest  string
	Cluster string
	Topic   string
}

func loadPrompts(dir string) (prompts, error) {
	if dir == "" {
		dir = "prompts"
	}
	p := prompts{}
	var err error
	if p.Summary, err = readPrompt(dir, promptSummary); err != nil {
		return prompts{}, err
	}
	if p.Digest, err = readPrompt(dir, promptDigest); err != nil {
		return prompts{}, err
	}
	if p.Cluster, err = readPrompt(dir, promptCluster); err != nil {
		return prompts{}, err
	}
	if p.Topic, err = readPrompt(dir, promptTopic); err != nil {
		return prompts{}, err
	}
	return p, nil
}

func readPrompt(dir, name string) (string, error) {
	path := filepath.Join(dir, name)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("digest: read prompt %s: %w", path, err)
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		return "", fmt.Errorf("digest: empty prompt %s", path)
	}
	return text, nil
}

// renderPrompt replaces {{key}} placeholders with values from vars.
func renderPrompt(tmpl string, vars map[string]string) string {
	out := tmpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", v)
	}
	return out
}
