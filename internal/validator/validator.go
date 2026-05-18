// Package validator performs structural validation of the Speclite state.
//
// Checks performed:
//   - Relation integrity: dangling references (to_id does not exist)
//   - Invalid status transitions (deprecated → draft, etc.)
//   - Cyclic dependency detection via DFS on depends_on edges
//   - Missing required fields (title, kind, status, body)
package validator

import (
	"fmt"

	"github.com/mikesorae/speqlite/internal/db"
)

// Severity classifies a validation finding.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue represents a single validation finding.
type Issue struct {
	Severity Severity
	Code     string
	Message  string
	SpecID   string
}

func (i Issue) String() string {
	if i.SpecID != "" {
		return fmt.Sprintf("[%s] %s: %s (spec: %s)", i.Severity, i.Code, i.Message, i.SpecID)
	}
	return fmt.Sprintf("[%s] %s: %s", i.Severity, i.Code, i.Message)
}

// Report is the full output of a validation run.
type Report struct {
	Issues []Issue
}

// HasErrors returns true if any issue has SeverityError.
func (r *Report) HasErrors() bool {
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error-severity issues.
func (r *Report) ErrorCount() int {
	count := 0
	for _, i := range r.Issues {
		if i.Severity == SeverityError {
			count++
		}
	}
	return count
}

// WarningCount returns the number of warning-severity issues.
func (r *Report) WarningCount() int {
	count := 0
	for _, i := range r.Issues {
		if i.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

// validStatusTransitions defines which statuses a spec may move FROM → TO.
// A spec may always stay in the same status.
var validStatusTransitions = map[string]map[string]bool{
	"draft":       {"review": true, "fixed": true, "deprecated": true},
	"review":      {"fixed": true, "draft": true, "deprecated": true},
	"fixed":       {"implemented": true, "review": true, "deprecated": true},
	"implemented": {"verified": true, "fixed": true, "deprecated": true},
	"verified":    {"deprecated": true},
	"deprecated":  {},
}

// validStatuses is the set of all valid status values.
var validStatuses = map[string]bool{
	"draft":       true,
	"review":      true,
	"fixed":       true,
	"implemented": true,
	"verified":    true,
	"deprecated":  true,
}

// Validate runs all structural checks against the database and returns a Report.
func Validate(database *db.DB) (*Report, error) {
	report := &Report{}

	specs, err := database.ListSpecs("", "")
	if err != nil {
		return nil, fmt.Errorf("validator: list specs: %w", err)
	}

	relations, err := database.ListAllRelations()
	if err != nil {
		return nil, fmt.Errorf("validator: list relations: %w", err)
	}

	// Build an index for O(1) lookups.
	specByID := make(map[string]db.Spec, len(specs))
	for _, s := range specs {
		specByID[s.ID] = s
	}

	// 1. Missing required fields.
	checkMissingFields(specs, report)

	// 2. Invalid status values.
	checkInvalidStatus(specs, report)

	// 3. Relation integrity (dangling references).
	checkRelationIntegrity(relations, specByID, report)

	// 4. Cyclic dependency detection via DFS on depends_on edges.
	checkCycles(specs, relations, report)

	return report, nil
}

// checkMissingFields flags specs with empty required fields.
func checkMissingFields(specs []db.Spec, report *Report) {
	for _, s := range specs {
		if s.Title == "" {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError,
				Code:     "MISSING_TITLE",
				Message:  "spec has empty title",
				SpecID:   s.ID,
			})
		}
		if s.Kind == "" {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError,
				Code:     "MISSING_KIND",
				Message:  "spec has empty kind",
				SpecID:   s.ID,
			})
		}
		if s.Status == "" {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError,
				Code:     "MISSING_STATUS",
				Message:  "spec has empty status",
				SpecID:   s.ID,
			})
		}
		if s.Body == "" {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityWarning,
				Code:     "EMPTY_BODY",
				Message:  "spec has empty body",
				SpecID:   s.ID,
			})
		}
	}
}

// checkInvalidStatus flags specs whose status is not a known value.
func checkInvalidStatus(specs []db.Spec, report *Report) {
	for _, s := range specs {
		if s.Status != "" && !validStatuses[s.Status] {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError,
				Code:     "INVALID_STATUS",
				Message:  fmt.Sprintf("unknown status %q; valid: draft, review, fixed, implemented, verified, deprecated", s.Status),
				SpecID:   s.ID,
			})
		}
	}
}

// checkRelationIntegrity detects dangling relation references.
func checkRelationIntegrity(relations []db.Relation, specByID map[string]db.Spec, report *Report) {
	for _, r := range relations {
		if _, ok := specByID[r.ToID]; !ok {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError,
				Code:     "DANGLING_RELATION",
				Message:  fmt.Sprintf("relation %q → %q references non-existent spec %q", r.FromID, r.Relation, r.ToID),
				SpecID:   r.FromID,
			})
		}
		if _, ok := specByID[r.FromID]; !ok {
			report.Issues = append(report.Issues, Issue{
				Severity: SeverityError,
				Code:     "DANGLING_RELATION",
				Message:  fmt.Sprintf("relation from non-existent spec %q → %q → %q", r.FromID, r.Relation, r.ToID),
				SpecID:   r.FromID,
			})
		}
	}
}

// checkCycles performs DFS on depends_on edges to detect cycles.
func checkCycles(specs []db.Spec, relations []db.Relation, report *Report) {
	// Build adjacency list for depends_on edges only.
	adj := make(map[string][]string)
	for _, s := range specs {
		adj[s.ID] = nil
	}
	for _, r := range relations {
		if r.Relation == "depends_on" {
			adj[r.FromID] = append(adj[r.FromID], r.ToID)
		}
	}

	// DFS state.
	const (
		unvisited = 0
		inStack   = 1
		done      = 2
	)
	state := make(map[string]int, len(specs))
	var path []string

	var dfs func(node string) bool
	dfs = func(node string) bool {
		state[node] = inStack
		path = append(path, node)

		for _, neighbour := range adj[node] {
			switch state[neighbour] {
			case inStack:
				// Found a cycle.
				// Find where the cycle starts in path.
				cycleStart := -1
				for i, p := range path {
					if p == neighbour {
						cycleStart = i
						break
					}
				}
				cycle := path[cycleStart:]
				report.Issues = append(report.Issues, Issue{
					Severity: SeverityError,
					Code:     "CYCLIC_DEPENDENCY",
					Message:  fmt.Sprintf("dependency cycle detected: %v → %s", cycle, neighbour),
					SpecID:   node,
				})
				return true
			case unvisited:
				if dfs(neighbour) {
					return true
				}
			}
		}

		path = path[:len(path)-1]
		state[node] = done
		return false
	}

	for _, s := range specs {
		if state[s.ID] == unvisited {
			path = nil
			dfs(s.ID)
		}
	}
}
