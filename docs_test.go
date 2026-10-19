package tgscreens_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	goBlock  = regexp.MustCompile("(?s)```go\n(.*?)```")
	mdLink   = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	nonSlug  = regexp.MustCompile(`[^a-z0-9_\- ]`)
	headingS = regexp.MustCompile(`^#{1,6}\s+(.*)$`)
)

// TestDocs_goBlocksAreCompiledSource keeps the written guides honest: every
// go code block in README.md and docs/ must be a verbatim excerpt, ignoring
// whitespace, of a Go file in this repository, so a snippet compiles because
// its source does. Use a text block for anything that is not real code.
func TestDocs_goBlocksAreCompiledSource(t *testing.T) {
	sources := goSources(t)

	for _, doc := range append([]string{"README.md"}, markdownIn(t, "docs")...) {
		for i, m := range goBlock.FindAllStringSubmatch(read(t, doc), -1) {
			block := normalize(m[1])
			found := false
			for _, src := range sources {
				if strings.Contains(src, block) {
					found = true
					break
				}
			}
			assert.True(t, found, "%s: go block %d is not an excerpt of any Go source:\n%s", doc, i+1, m[1])
		}
	}
}

// TestDocs_relativeLinksResolve checks that every relative Markdown link points
// at a file that exists and, when it has a fragment, at a heading that exists.
func TestDocs_relativeLinksResolve(t *testing.T) {
	roots, err := filepath.Glob("*.md")
	require.NoError(t, err)

	for _, doc := range append(roots, markdownIn(t, "docs")...) {
		for _, m := range mdLink.FindAllStringSubmatch(read(t, doc), -1) {
			target := m[1]
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}

			path, fragment, _ := strings.Cut(target, "#")
			resolved := doc
			if path != "" {
				resolved = filepath.Join(filepath.Dir(doc), filepath.FromSlash(path))
			}
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("%s: link %q does not resolve: %v", doc, target, err)
				continue
			}
			if fragment != "" && strings.HasSuffix(resolved, ".md") {
				assert.Contains(t, headingSlugs(read(t, resolved)), fragment, "%s: link %q has no such heading", doc, target)
			}
		}
	}
}

func goSources(t *testing.T) []string {
	t.Helper()

	var sources []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != "." {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			sources = append(sources, normalize(read(t, path)))
		}
		return nil
	})
	require.NoError(t, err)
	return sources
}

func markdownIn(t *testing.T, dir string) []string {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(dir, "*.md"))
	require.NoError(t, err)
	return files
}

func read(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

// headingSlugs returns the GitHub-style anchors of a document's headings,
// skipping lines inside code fences.
func headingSlugs(doc string) []string {
	var slugs []string
	inFence := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := headingS.FindStringSubmatch(line); m != nil {
			slug := nonSlug.ReplaceAllString(strings.ToLower(strings.TrimSpace(m[1])), "")
			slugs = append(slugs, strings.ReplaceAll(slug, " ", "-"))
		}
	}
	return slugs
}
