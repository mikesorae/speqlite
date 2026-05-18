// Package normalizer converts raw importer output into NormalizedSpec structs
// suitable for diffing against the SQLite state.
package normalizer

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"

	"github.com/speclite/speclite/internal/importer"
)

// NormalizedSpec is a fully normalised specification ready for plan/apply.
type NormalizedSpec struct {
	// ID is the canonical identifier. Generated from slug if not found inline.
	ID string
	// Title is the cleaned spec title.
	Title string
	// Kind is the inferred spec kind: requirement, command, state, constraint.
	Kind string
	// Status defaults to "draft" for all newly imported specs.
	Status string
	// Body is the raw body text.
	Body string
	// Hash is the SHA-256 of the canonical fields.
	Hash string
	// Relations are the dependency hints discovered in the body.
	Relations []importer.RelationHint
}

// kindPatterns maps compiled regexps to kind labels.
// They are evaluated in order; the first match wins.
var kindPatterns = []struct {
	re   *regexp.Regexp
	kind string
}{
	// Command specs: IDs starting with CMD- or titles containing "command".
	{regexp.MustCompile(`(?i)^CMD-|\bcommand\b`), "command"},
	// State specs: IDs starting with STATE- or titles containing "state".
	{regexp.MustCompile(`(?i)^STATE-|\bstate\b`), "state"},
	// Constraint specs: IDs starting with CON- or CONSTRAINT- or titles containing "constraint".
	{regexp.MustCompile(`(?i)^CON-|^CONSTRAINT-|\bconstraint\b`), "constraint"},
	// Requirement specs: IDs starting with FR-, REQ-, or titles containing "requirement|shall|must".
	{regexp.MustCompile(`(?i)^FR-|^REQ-|\brequirement\b|\bshall\b|\bmust\b`), "requirement"},
}

// Normalize converts a slice of RawSpec values from a single import into
// NormalizedSpec values. Duplicate IDs within the batch are disambiguated
// by appending a numeric suffix.
func Normalize(raws []importer.RawSpec) []NormalizedSpec {
	seen := map[string]int{} // tracks how many times each base ID has been used
	out := make([]NormalizedSpec, 0, len(raws))

	for _, raw := range raws {
		ns := normalizeOne(raw)
		// Disambiguate duplicate IDs within the same batch.
		base := ns.ID
		if count, exists := seen[base]; exists {
			seen[base] = count + 1
			ns.ID = base + "-" + intStr(count+1)
		} else {
			seen[base] = 0
		}
		ns.Hash = canonicalHash(ns)
		out = append(out, ns)
	}

	return out
}

// normalizeOne processes a single RawSpec into a NormalizedSpec.
func normalizeOne(raw importer.RawSpec) NormalizedSpec {
	title := cleanTitle(raw.Title)
	if title == "" {
		// Fall back to the first non-empty line of the body.
		title = firstLine(raw.Body)
	}

	id := raw.InlineID
	if id == "" {
		id = slugToID(title)
	}
	if id == "" {
		id = "SPEC-UNKNOWN"
	}

	kind := inferKind(id, title)
	body := strings.TrimSpace(raw.Body)

	return NormalizedSpec{
		ID:        id,
		Title:     title,
		Kind:      kind,
		Status:    "draft",
		Body:      body,
		Relations: raw.Relations,
	}
}

// inferKind attempts to determine the spec kind from the ID and title.
// Returns "requirement" as the default if no pattern matches.
func inferKind(id, title string) string {
	combined := id + " " + title
	for _, p := range kindPatterns {
		if p.re.MatchString(combined) {
			return p.kind
		}
	}
	return "requirement"
}

// cleanTitle strips leading Markdown heading markers and excess whitespace.
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	// Strip leading # characters (Markdown headings that goldmark didn't strip).
	s = strings.TrimLeft(s, "#")
	s = strings.TrimSpace(s)
	return s
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

// slugToID converts a human-readable title into a slug-style spec ID.
// Example: "Import Command" → "IMPORT-COMMAND"
//
// Rules:
//   - Uppercase all runes.
//   - Replace non-alphanumeric sequences with a single hyphen.
//   - Strip leading/trailing hyphens.
//   - Truncate to 40 characters to keep IDs manageable.
func slugToID(title string) string {
	if title == "" {
		return ""
	}

	var buf strings.Builder
	prevHyphen := true // suppress leading hyphens

	for _, r := range strings.ToUpper(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			buf.WriteRune(r)
			prevHyphen = false
		} else {
			if !prevHyphen {
				buf.WriteByte('-')
				prevHyphen = true
			}
		}
	}

	slug := strings.TrimRight(buf.String(), "-")
	if len(slug) > 40 {
		slug = slug[:40]
		// Don't end with a hyphen after truncation.
		slug = strings.TrimRight(slug, "-")
	}
	return slug
}

// canonicalHash returns the SHA-256 of the canonical string representation.
// Format: "{id}\n{title}\n{kind}\n{status}\n{body}"
func canonicalHash(ns NormalizedSpec) string {
	s := ns.ID + "\n" + ns.Title + "\n" + ns.Kind + "\n" + ns.Status + "\n" + ns.Body
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// intStr converts an integer to its decimal string representation without
// importing strconv (to keep dependencies minimal here).
func intStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
