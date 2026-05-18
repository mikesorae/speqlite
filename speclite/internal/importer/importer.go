// Package importer parses Markdown and plain-text files into raw spec candidates.
// It does NOT mutate any state — it only extracts structured data from input.
package importer

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// RelationHint represents a discovered dependency hint within a spec body.
type RelationHint struct {
	// Kind is the relation type (depends_on, implements, verifies, supersedes, related_to).
	Kind string
	// TargetID is the referenced spec identifier.
	TargetID string
}

// RawSpec is the unprocessed output of the importer for a single spec candidate.
// It preserves everything discovered from the source text before normalisation.
type RawSpec struct {
	// Title is the heading text or first non-empty line.
	Title string
	// InlineID is a discovered explicit ID (e.g. CMD-IMPORT, FR-1) or empty if none found.
	InlineID string
	// Body is the full text content associated with this spec candidate.
	Body string
	// Relations holds any dependency hints found in the body.
	Relations []RelationHint
}

// Result is the full output of an import operation on a single file.
type Result struct {
	// Specs contains all extracted raw spec candidates.
	Specs []RawSpec
	// SourceFile is the originating file path (for diagnostics).
	SourceFile string
}

// inlineIDRe matches explicit spec IDs embedded in text.
// Patterns: CMD-IMPORT, FR-1, STATE-2, REQ-42, etc.
// Must be at a word boundary to avoid false positives inside longer words.
var inlineIDRe = regexp.MustCompile(`\b([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+)\b`)

// relationKeywords maps textual keywords to canonical relation types.
var relationKeywords = map[string]string{
	"depends_on":  "depends_on",
	"depends on":  "depends_on",
	"implements":  "implements",
	"verifies":    "verifies",
	"supersedes":  "supersedes",
	"related_to":  "related_to",
	"related to":  "related_to",
}

// relationHintRe matches lines like "depends_on: CMD-FOO" or "implements CMD-BAR".
// It supports both colon-separated and space-separated forms.
var relationHintRe = regexp.MustCompile(
	`(?i)(depends[_ ]on|implements|verifies|supersedes|related[_ ]to)[:\s]+([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+)`,
)

// ParseFile parses content (Markdown or plain text) from sourceFile and returns
// a Result containing all discovered RawSpec candidates.
func ParseFile(sourceFile string, content []byte) (*Result, error) {
	// Try Markdown-aware parsing first. If no headings are found, fall back to
	// plain-text line scanning.
	specs := parseMarkdown(content)
	if len(specs) == 0 {
		specs = parsePlainText(content)
	}

	return &Result{
		Specs:      specs,
		SourceFile: sourceFile,
	}, nil
}

// parseMarkdown walks the goldmark AST and extracts one RawSpec per heading.
// Each heading starts a new spec; its body accumulates until the next same-or-higher heading.
func parseMarkdown(src []byte) []RawSpec {
	md := goldmark.New()
	reader := text.NewReader(src)
	doc := md.Parser().Parse(reader)

	var specs []RawSpec
	var current *RawSpec

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch node := n.(type) {
		case *ast.Heading:
			// Commit any in-progress spec.
			if current != nil {
				current.Relations = extractRelations(current.Body)
				specs = append(specs, *current)
			}
			// Start a new spec from the heading text.
			title := extractText(node, src)
			current = &RawSpec{
				Title:    title,
				InlineID: extractInlineID(title),
			}

		case *ast.Paragraph, *ast.CodeBlock, *ast.FencedCodeBlock, *ast.Blockquote,
			*ast.List, *ast.ListItem, *ast.TextBlock:
			if current == nil {
				// Content before the first heading — create an anonymous spec.
				current = &RawSpec{}
			}
			text := extractNodeText(node, src)
			if text != "" {
				if current.Body != "" {
					current.Body += "\n\n"
				}
				current.Body += text
				// Pick up inline ID from body if not already found in title.
				if current.InlineID == "" {
					current.InlineID = extractInlineID(text)
				}
			}
		}
		return ast.WalkContinue, nil
	})

	// Commit the last in-progress spec.
	if current != nil && (current.Title != "" || current.Body != "") {
		current.Relations = extractRelations(current.Body)
		specs = append(specs, *current)
	}

	return specs
}

// parsePlainText handles files with no headings by splitting on double newlines
// (paragraph breaks). Each non-empty paragraph becomes a RawSpec candidate.
func parsePlainText(src []byte) []RawSpec {
	paragraphs := bytes.Split(src, []byte("\n\n"))
	var specs []RawSpec

	for _, para := range paragraphs {
		trimmed := strings.TrimSpace(string(para))
		if trimmed == "" {
			continue
		}
		lines := strings.SplitN(trimmed, "\n", 2)
		title := strings.TrimSpace(lines[0])
		body := trimmed
		if len(lines) > 1 {
			body = strings.TrimSpace(lines[1])
		} else {
			body = ""
		}

		spec := RawSpec{
			Title:    title,
			InlineID: extractInlineID(trimmed),
			Body:     body,
		}
		spec.Relations = extractRelations(trimmed)
		specs = append(specs, spec)
	}

	return specs
}

// extractText returns the plain-text content of an AST node (typically a heading).
func extractText(n ast.Node, src []byte) string {
	var buf bytes.Buffer
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if t, ok := child.(*ast.Text); ok {
			buf.Write(t.Segment.Value(src))
		} else if s, ok := child.(*ast.String); ok {
			buf.Write(s.Value)
		}
	}
	return strings.TrimSpace(buf.String())
}

// extractNodeText returns the plain-text representation of a block node's children.
func extractNodeText(n ast.Node, src []byte) string {
	var buf bytes.Buffer
	ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		// Only collect leaf text nodes to avoid double-counting.
		if t, ok := child.(*ast.Text); ok {
			segment := t.Segment.Value(src)
			buf.Write(segment)
			if t.SoftLineBreak() || t.HardLineBreak() {
				buf.WriteByte('\n')
			}
		} else if s, ok := child.(*ast.String); ok {
			buf.Write(s.Value)
		} else if child.Kind() == ast.KindCodeSpan {
			// Render code spans inline.
			for c := child.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					buf.Write(t.Segment.Value(src))
				}
			}
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(buf.String())
}

// extractInlineID finds the first explicit spec ID in text (e.g. CMD-IMPORT, FR-1).
func extractInlineID(text string) string {
	match := inlineIDRe.FindString(text)
	return match
}

// extractRelations scans body text for relation hints and returns all found hints.
func extractRelations(body string) []RelationHint {
	matches := relationHintRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	var hints []RelationHint
	seen := map[string]bool{}

	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		rawKind := strings.ToLower(strings.TrimRight(m[1], ":"))
		rawKind = strings.ReplaceAll(rawKind, " ", "_")
		targetID := strings.ToUpper(m[2])

		// Normalise keyword to canonical relation type.
		kind, ok := relationKeywords[rawKind]
		if !ok {
			// Try direct lookup after space→underscore normalisation.
			kind = rawKind
		}

		key := kind + ":" + targetID
		if !seen[key] {
			seen[key] = true
			hints = append(hints, RelationHint{Kind: kind, TargetID: targetID})
		}
	}

	return hints
}
