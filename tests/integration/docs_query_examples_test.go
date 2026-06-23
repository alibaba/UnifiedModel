package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/alibaba/UnifiedModel/internal/bootstrap"
	"github.com/alibaba/UnifiedModel/pkg/model"
)

type docQueryExample struct {
	Path    string
	Command string
	Action  string
	Query   string
}

func TestDocsQueryExamplesExecute(t *testing.T) {
	ctx := context.Background()
	app := bootstrap.NewMemoryApp(t.TempDir())
	// Public docs that use workspace "demo" are expected to run against the
	// bundled multi-domain quickstart sample loaded here.
	if _, err := app.LoadQuickStart(ctx, bootstrap.QuickStartOptions{}); err != nil {
		t.Fatalf("load quickstart: %v", err)
	}

	examples := collectDocQueryExamples(t, repoRoot(t))
	if len(examples) == 0 {
		t.Fatalf("expected documented query examples")
	}

	for _, example := range examples {
		example := example
		t.Run(example.Path+":"+example.Action+":"+example.Query, func(t *testing.T) {
			switch example.Action {
			case "run":
				if _, err := app.Query.Execute(ctx, "demo", model.QueryRequest{Query: example.Query}); err != nil {
					t.Fatalf("execute documented query from %s\ncommand: %s\nquery: %s\nerror: %v", example.Path, example.Command, example.Query, err)
				}
			case "explain":
				explain, err := app.Query.Explain(ctx, "demo", model.QueryRequest{Query: example.Query})
				if err != nil {
					t.Fatalf("explain documented query from %s\ncommand: %s\nquery: %s\nerror: %v", example.Path, example.Command, example.Query, err)
				}
				if explain.Source == "" || explain.Provider == "" {
					t.Fatalf("explain should include source and provider: %+v", explain)
				}
			default:
				t.Fatalf("unexpected action %q", example.Action)
			}
		})
	}
}

func TestExtractDocQueryCommands(t *testing.T) {
	markdown := strings.Join([]string{
		"```bash",
		`go run ./cmd/umctl --addr http://localhost:8080 query run demo ".umodel | limit 5"`,
		`umctl query explain demo ".entity with(domain='devops', name='devops.service') | limit 5" -o json`,
		"```",
		"```bash",
		`go run ./cmd/umctl --addr http://localhost:8080 query run demo \`,
		`  ".topo | graph-call cypher(` + "`MATCH (src)-[r]->(dest) RETURN src, r AS relation, dest LIMIT 20`" + `)"`,
		"```",
		"```json",
		`{"query": ".umodel | limit 5"}`,
		"```",
	}, "\n")

	examples := extractDocQueryExamples("doc.md", markdown)
	if len(examples) != 3 {
		t.Fatalf("expected 3 examples, got %d: %+v", len(examples), examples)
	}
	if examples[0].Action != "run" || examples[0].Query != ".umodel | limit 5" {
		t.Fatalf("unexpected first example: %+v", examples[0])
	}
	if examples[1].Action != "explain" || examples[1].Query != ".entity with(domain='devops', name='devops.service') | limit 5" {
		t.Fatalf("unexpected second example: %+v", examples[1])
	}
	if examples[2].Action != "run" || !strings.Contains(examples[2].Query, "graph-call cypher") {
		t.Fatalf("unexpected multiline example: %+v", examples[2])
	}
}

func collectDocQueryExamples(t *testing.T, root string) []docQueryExample {
	t.Helper()
	var paths []string
	for _, rel := range []string{
		filepath.Join("docs", "en"),
		filepath.Join("docs", "zh"),
		filepath.Join("examples", "quickstart-multidomain"),
	} {
		abs := filepath.Join(root, rel)
		err := filepath.WalkDir(abs, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path != abs && rel == filepath.Join("examples", "quickstart-multidomain") && filepath.Base(path) != "quickstart-multidomain" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(entry.Name(), ".md") {
				return nil
			}
			if rel == filepath.Join("examples", "quickstart-multidomain") && !strings.HasPrefix(entry.Name(), "README") {
				return nil
			}
			paths = append(paths, path)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", abs, err)
		}
	}
	sort.Strings(paths)

	var examples []docQueryExample
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("rel %s: %v", path, err)
		}
		examples = append(examples, extractDocQueryExamples(filepath.ToSlash(rel), string(body))...)
	}
	return examples
}

func extractDocQueryExamples(path string, markdown string) []docQueryExample {
	var examples []docQueryExample
	for _, block := range shellCodeBlocks(markdown) {
		for _, command := range shellCommands(block) {
			action, query, ok := parseDocQueryCommand(command)
			if !ok {
				continue
			}
			examples = append(examples, docQueryExample{
				Path:    path,
				Command: command,
				Action:  action,
				Query:   query,
			})
		}
	}
	return examples
}

func shellCodeBlocks(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	var blocks []string
	inFence := false
	shellFence := false
	var current []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				if shellFence {
					blocks = append(blocks, strings.Join(current, "\n"))
				}
				inFence = false
				shellFence = false
				current = nil
				continue
			}
			info := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trimmed, "```")))
			inFence = true
			shellFence = info == "" || info == "bash" || info == "sh" || info == "shell" || info == "console"
			current = nil
			continue
		}
		if inFence && shellFence {
			current = append(current, line)
		}
	}
	return blocks
}

func shellCommands(block string) []string {
	var commands []string
	var current strings.Builder
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		if strings.HasSuffix(line, "\\") {
			current.WriteString(strings.TrimSpace(strings.TrimSuffix(line, "\\")))
			continue
		}
		current.WriteString(line)
		commands = append(commands, strings.TrimSpace(current.String()))
		current.Reset()
	}
	if current.Len() > 0 {
		commands = append(commands, strings.TrimSpace(current.String()))
	}
	return commands
}

func parseDocQueryCommand(command string) (string, string, bool) {
	fields := strings.Fields(command)
	for i := 0; i+3 < len(fields); i++ {
		if fields[i] != "query" || (fields[i+1] != "run" && fields[i+1] != "explain") || fields[i+2] != "demo" {
			continue
		}
		query, ok := quotedArgumentAfter(command, fields[i+2])
		if !ok {
			return "", "", false
		}
		return fields[i+1], query, true
	}
	return "", "", false
}

func quotedArgumentAfter(command string, workspace string) (string, bool) {
	needle := " " + workspace
	start := strings.Index(command, needle)
	if start < 0 {
		return "", false
	}
	start += len(needle)
	for start < len(command) && command[start] == ' ' {
		start++
	}
	if start >= len(command) || (command[start] != '"' && command[start] != '\'') {
		return "", false
	}
	quote := command[start]
	var out strings.Builder
	escaped := false
	for i := start + 1; i < len(command); i++ {
		ch := command[i]
		if escaped {
			out.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == quote {
			return out.String(), true
		}
		out.WriteByte(ch)
	}
	return "", false
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}
