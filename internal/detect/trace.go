package detect

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PredOutcome is the exported result of a predicate evaluation.
type PredOutcome string

const (
	OutcomePass    PredOutcome = "pass"
	OutcomeFail    PredOutcome = "fail"
	OutcomeMissing PredOutcome = "missing"
	OutcomeParse   PredOutcome = "parse"
)

// TraceNode is one node in a predicate pass/fail tree.
type TraceNode struct {
	Label    string
	Outcome  PredOutcome
	Children []*TraceNode
}

func (r predResult) outcome() PredOutcome {
	switch r {
	case predPass:
		return OutcomePass
	case predMissing:
		return OutcomeMissing
	case predParse:
		return OutcomeParse
	default:
		return OutcomeFail
	}
}

// EvalTrace evaluates p and returns a pass/fail tree for detectors test.
func (p *Predicate) EvalTrace(ctx *Context, dir string) *TraceNode {
	if p == nil {
		return &TraceNode{Label: "<nil>", Outcome: OutcomeFail}
	}
	return p.trace(ctx, dir)
}

func (p *Predicate) trace(ctx *Context, dir string) *TraceNode {
	switch {
	case p.FileExists != "":
		r := existsFile(ctx, filepath.Join(dir, p.FileExists))
		return &TraceNode{Label: fmt.Sprintf("file_exists: %s", p.FileExists), Outcome: r.outcome()}
	case p.DirExists != "":
		r := existsDir(ctx, filepath.Join(dir, p.DirExists))
		return &TraceNode{Label: fmt.Sprintf("dir_exists: %s", p.DirExists), Outcome: r.outcome()}
	case p.GlobExists != "":
		r := existsGlob(ctx, dir, p.GlobExists)
		return &TraceNode{Label: fmt.Sprintf("glob_exists: %s", p.GlobExists), Outcome: r.outcome()}
	case p.FileContains != nil:
		r := evalFileContains(ctx, dir, p.FileContains)
		return &TraceNode{
			Label:   fmt.Sprintf("file_contains: %s ~ %s", p.FileContains.File, p.FileContains.Pattern),
			Outcome: r.outcome(),
		}
	case p.JSONPath != nil:
		r := evalJSONPath(ctx, dir, p.JSONPath)
		return &TraceNode{
			Label:   fmt.Sprintf("json_path: %s %s", p.JSONPath.File, p.JSONPath.Path),
			Outcome: r.outcome(),
		}
	case p.YAMLPath != nil:
		r := evalYAMLPath(ctx, dir, p.YAMLPath)
		return &TraceNode{
			Label:   fmt.Sprintf("yaml_path: %s %s", p.YAMLPath.File, p.YAMLPath.Path),
			Outcome: r.outcome(),
		}
	case p.TOMLPath != nil:
		r := evalTOMLPath(ctx, dir, p.TOMLPath)
		return &TraceNode{
			Label:   fmt.Sprintf("toml_path: %s %s", p.TOMLPath.File, p.TOMLPath.Path),
			Outcome: r.outcome(),
		}
	case len(p.Any) > 0:
		n := &TraceNode{Label: "any"}
		sawParse := false
		pass := false
		for i := range p.Any {
			child := p.Any[i].trace(ctx, dir)
			n.Children = append(n.Children, child)
			switch child.Outcome {
			case OutcomePass:
				pass = true
			case OutcomeParse:
				sawParse = true
			}
		}
		switch {
		case pass:
			n.Outcome = OutcomePass
		case sawParse:
			n.Outcome = OutcomeParse
		default:
			n.Outcome = OutcomeFail
		}
		return n
	case len(p.All) > 0:
		n := &TraceNode{Label: "all"}
		sawParse := false
		fail := false
		var failOutcome PredOutcome
		for i := range p.All {
			child := p.All[i].trace(ctx, dir)
			n.Children = append(n.Children, child)
			switch child.Outcome {
			case OutcomePass:
				continue
			case OutcomeParse:
				sawParse = true
			default:
				if !fail {
					fail = true
					failOutcome = child.Outcome
				}
			}
		}
		switch {
		case fail:
			n.Outcome = failOutcome
		case sawParse:
			n.Outcome = OutcomeParse
		default:
			n.Outcome = OutcomePass
		}
		return n
	case p.Not != nil:
		child := p.Not.trace(ctx, dir)
		n := &TraceNode{Label: "not", Children: []*TraceNode{child}}
		switch child.Outcome {
		case OutcomePass:
			n.Outcome = OutcomeFail
		case OutcomeParse:
			n.Outcome = OutcomeParse
		default:
			n.Outcome = OutcomePass
		}
		return n
	default:
		return &TraceNode{Label: "<empty>", Outcome: OutcomeFail}
	}
}

// FormatTrace renders a trace tree as indented text.
func FormatTrace(n *TraceNode, indent string) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s[%s] %s\n", indent, n.Outcome, n.Label)
	for _, c := range n.Children {
		b.WriteString(FormatTrace(c, indent+"  "))
	}
	return b.String()
}
