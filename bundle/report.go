package bundle

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anyproto/any-block/codec/anyblockjson"
)

// Severity grades a bundle-level issue (§12). Only SeverityError makes
// Validate refuse; the other two are what Inspect exists to carry.
type Severity uint8

const (
	// SeverityError: the bundle is not valid AnyBlock. Validate refuses it.
	SeverityError Severity = iota
	// SeverityWarning: the bundle is valid and states a loss — a target the
	// space had no row for, or one it holds that this export did not write.
	SeverityWarning
	// SeverityInfo: the bundle is valid and states something that is not a
	// loss — a target the space deleted, by design.
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	}
	return fmt.Sprintf("severity(%d)", uint8(s))
}

// ReportIssue is one bundle-level verdict: a severity, a stable code where one
// exists (callers branch on Code, never on Message), the index field or
// bundle path it is about, and the wording Validate has always used.
type ReportIssue struct {
	Severity Severity
	Code     anyblockjson.IssueCode
	Path     string
	Message  string
}

// Report is everything one walk of a bundle had to say, errors first, then
// warnings, then info, each group sorted so two walks of one bundle read
// alike.
type Report struct {
	Issues []ReportIssue
}

// Errors returns the error-severity issues — exactly what Validate refuses
// on.
func (r *Report) Errors() []ReportIssue {
	var out []ReportIssue
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			out = append(out, issue)
		}
	}
	return out
}

// Err is Validate's verdict for this report: nil when no issue is an error,
// else the same joined, sorted message Validate has always returned.
func (r *Report) Err() error {
	errs := r.Errors()
	if len(errs) == 0 {
		return nil
	}
	messages := make([]string, 0, len(errs))
	for _, issue := range errs {
		messages = append(messages, issue.Message)
	}
	sort.Strings(messages)
	return fmt.Errorf("bundle validation failed:\n- %s", strings.Join(messages, "\n- "))
}

func (r *Report) add(issue ReportIssue) { r.Issues = append(r.Issues, issue) }

func (r *Report) sorted() {
	sort.SliceStable(r.Issues, func(i, j int) bool {
		a, b := r.Issues[i], r.Issues[j]
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Message < b.Message
	})
}
