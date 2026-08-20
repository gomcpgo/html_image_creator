package frames

import (
	"fmt"
	"sort"
	"strings"
)

const (
	canvasTolerance = 0.5  // px slack for in_canvas (sub-pixel rounding)
	overlapMin      = 4.0  // boxes must intersect by more than this in BOTH axes (slivers/grazes pass)
	inkInkMin       = 10.0 // text-vs-text minimum: line boxes include font leading, which
	//                        overstates vertical ink by single-digit px; real text collisions
	//                        overlap by tens of px
	movedEpsilon = 0.5 // px change that counts as an untouched element moving
)

// pairMin returns the intrusion threshold for a pair of elements.
func pairMin(a, b *Elem) float64 {
	if a.Ink != nil && b.Ink != nil {
		return inkInkMin
	}
	return overlapMin
}

// SelTable maps every selector any rule (or report_boxes) uses to a stable
// index; the in-page JS reports, per element, which indices it matches.
type SelTable struct {
	Sels []string
	idx  map[string]int
	// watch[i] = true if selector i must match at least once across the run
	// (rule + report_boxes selectors; no_overlap allow pairs are exempt).
	watch []bool
}

// LineSelIdx returns indices of selectors whose matches need line counts.
func (t *SelTable) LineSelIdx(spec *Spec) []int {
	out := []int{} // must marshal to [] not null: the in-page JS calls .includes on it
	for _, r := range spec.Validations {
		if r.Type == "max_lines" {
			out = append(out, t.idx[r.Selector])
		}
	}
	return out
}

func BuildSelTable(spec *Spec) *SelTable {
	t := &SelTable{idx: map[string]int{}}
	add := func(sel string, watched bool) {
		if i, ok := t.idx[sel]; ok {
			if watched {
				t.watch[i] = true
			}
			return
		}
		t.idx[sel] = len(t.Sels)
		t.Sels = append(t.Sels, sel)
		t.watch = append(t.watch, watched)
	}
	for _, r := range spec.Validations {
		for _, s := range r.Selectors {
			add(s, true)
		}
		for _, s := range r.Among {
			add(s, true)
		}
		for _, p := range r.Allow {
			add(p[0], false)
			add(p[1], false)
		}
		if r.Selector != "" {
			add(r.Selector, true)
		}
		for _, s := range r.Zones {
			add(s, true)
		}
	}
	for _, s := range spec.ReportBoxes {
		add(s, true)
	}
	return t
}

// Evaluate runs all rules over the collected measurements and assembles the report.
func Evaluate(spec *Spec, table *SelTable, scenes []SceneFrames) *Report {
	rep := &Report{}
	matchedEver := make([]bool, len(table.Sels))

	for _, sf := range scenes {
		sr := SceneResult{Name: sf.Name, Frames: len(sf.Frames), Exported: sf.Exported}
		// key -> index into sr.Violations, for consecutive-frame dedupe
		open := map[string]int{}
		var prev map[string][4]float64
		for fi, els := range sf.Frames {
			frameNo := fi + 1
			bySel := groupBySelector(els, len(table.Sels), matchedEver)
			raw := frameViolations(spec, table, els, bySel, prev, frameNo == 1)
			seen := map[string]bool{}
			for _, v := range raw {
				key := v.Rule + "|" + strings.Join(v.Elements, "|")
				if seen[key] {
					continue
				}
				seen[key] = true
				if i, ok := open[key]; ok && sr.Violations[i].LastFrame == frameNo-1 {
					sr.Violations[i].LastFrame = frameNo
					continue
				}
				sr.Violations = append(sr.Violations, Violation{
					Rule: v.Rule, FirstFrame: frameNo, LastFrame: frameNo,
					Elements: v.Elements, Detail: v.Detail,
				})
				open[key] = len(sr.Violations) - 1
			}
			prev = rectsByPath(els)
		}
		rep.Totals.Violations += len(sr.Violations)
		rep.Scenes = append(rep.Scenes, sr)
		rep.Totals.Frames += len(sf.Frames)
	}
	rep.Totals.Scenes = len(scenes)

	for i, watched := range table.watch {
		if watched && !matchedEver[i] {
			rep.CoverageErrors = append(rep.CoverageErrors,
				fmt.Sprintf("selector %q matched no element in any frame — rule watches nothing", table.Sels[i]))
		}
	}
	rep.Totals.Violations += len(rep.CoverageErrors)

	if len(spec.ReportBoxes) > 0 {
		rep.Boxes = reportBoxes(spec, table, scenes)
	}

	if rep.Totals.Violations > 0 {
		rep.Status = "violations_found"
	} else {
		rep.Status = "succeeded"
	}
	return rep
}

type rawViolation struct {
	Rule     string
	Elements []string
	Detail   string
}

func groupBySelector(els []Elem, n int, matchedEver []bool) [][]*Elem {
	bySel := make([][]*Elem, n)
	for i := range els {
		for _, si := range els[i].Sels {
			if si >= 0 && si < n {
				bySel[si] = append(bySel[si], &els[i])
				matchedEver[si] = true
			}
		}
	}
	return bySel
}

func rectsByPath(els []Elem) map[string][4]float64 {
	m := make(map[string][4]float64, len(els))
	for _, e := range els {
		m[e.Path] = e.Rect
	}
	return m
}

func frameViolations(spec *Spec, table *SelTable, els []Elem, bySel [][]*Elem,
	prev map[string][4]float64, firstFrame bool) []rawViolation {
	var out []rawViolation
	parents := make(map[string]string, len(els))
	for _, e := range els {
		parents[e.Path] = e.Parent
	}

	for _, r := range spec.Validations {
		switch r.Type {
		case "in_canvas":
			for _, s := range r.Selectors {
				for _, e := range bySel[table.idx[s]] {
					if d := canvasOverrun(e.VBox(), spec.Width, spec.Height); d != "" {
						out = append(out, rawViolation{"in_canvas", []string{e.Path}, d})
					}
				}
			}
		case "no_overlap":
			members := dedupeByPath(collect(bySel, table, r.Among))
			for i := 0; i < len(members); i++ {
				for j := i + 1; j < len(members); j++ {
					a, b := members[i], members[j]
					if isAncestor(parents, a.Path, b.Path) || isAncestor(parents, b.Path, a.Path) {
						continue
					}
					if pairAllowed(a, b, r.Allow, table) {
						continue
					}
					if w, h, x, y := intersect(a.VBox(), b.VBox()); w > pairMin(a, b) && h > pairMin(a, b) {
						out = append(out, rawViolation{"no_overlap",
							sortedPair(a.Path, b.Path),
							fmt.Sprintf("boxes overlap %.0fx%.0fpx at (%.0f,%.0f)", w, h, x, y)})
					}
				}
			}
		case "keep_out":
			zones := dedupeByPath(collect(bySel, table, r.Zones))
			for _, e := range bySel[table.idx[r.Selector]] {
				for _, z := range zones {
					if e.Path == z.Path || isAncestor(parents, e.Path, z.Path) || isAncestor(parents, z.Path, e.Path) {
						continue
					}
					if w, h, x, y := intersect(e.VBox(), z.VBox()); w > pairMin(e, z) && h > pairMin(e, z) {
						out = append(out, rawViolation{"keep_out",
							[]string{e.Path, z.Path},
							fmt.Sprintf("enters zone by %.0fx%.0fpx at (%.0f,%.0f)", w, h, x, y)})
					}
				}
			}
		case "max_lines":
			for _, e := range bySel[table.idx[r.Selector]] {
				if e.Lines > r.Lines {
					out = append(out, rawViolation{"max_lines", []string{e.Path},
						fmt.Sprintf("renders %d lines, max %d (text wrapped)", e.Lines, r.Lines)})
				}
			}
		}
	}

	// Implicit rule: images must load. A broken image is a layout defect the
	// geometric rules cannot see — the broken-icon box is nothing like the
	// intended box.
	for _, e := range els {
		if e.Broken != "" {
			out = append(out, rawViolation{"broken_image", []string{e.Path},
				fmt.Sprintf("image failed to load: %s", e.Broken)})
		}
	}

	// Implicit rule: untouched elements must not move between consecutive frames.
	if !firstFrame && prev != nil {
		for _, e := range els {
			if e.Touched {
				continue
			}
			p, ok := prev[e.Path]
			if !ok {
				continue
			}
			if maxDelta(p, e.Rect) > movedEpsilon {
				out = append(out, rawViolation{"moved", []string{e.Path},
					fmt.Sprintf("box changed (%.0f,%.0f %.0fx%.0f) -> (%.0f,%.0f %.0fx%.0f) without being targeted by a delta",
						p[0], p[1], p[2], p[3], e.Rect[0], e.Rect[1], e.Rect[2], e.Rect[3])})
			}
		}
	}
	return out
}

func collect(bySel [][]*Elem, table *SelTable, sels []string) []*Elem {
	var out []*Elem
	for _, s := range sels {
		out = append(out, bySel[table.idx[s]]...)
	}
	return out
}

func dedupeByPath(els []*Elem) []*Elem {
	seen := map[string]bool{}
	var out []*Elem
	for _, e := range els {
		if !seen[e.Path] {
			seen[e.Path] = true
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// isAncestor reports whether a is an ancestor of b, via the parent-path map.
func isAncestor(parents map[string]string, a, b string) bool {
	for p, ok := parents[b]; ok && p != ""; p, ok = parents[p] {
		if p == a {
			return true
		}
	}
	return false
}

func pairAllowed(a, b *Elem, allow [][]string, table *SelTable) bool {
	for _, p := range allow {
		i1, i2 := table.idx[p[0]], table.idx[p[1]]
		if (hasSel(a, i1) && hasSel(b, i2)) || (hasSel(b, i1) && hasSel(a, i2)) {
			return true
		}
	}
	return false
}

func hasSel(e *Elem, idx int) bool {
	for _, s := range e.Sels {
		if s == idx {
			return true
		}
	}
	return false
}

func intersect(a, b [4]float64) (w, h, x, y float64) {
	x = maxF(a[0], b[0])
	y = maxF(a[1], b[1])
	w = minF(a[0]+a[2], b[0]+b[2]) - x
	h = minF(a[1]+a[3], b[1]+b[3]) - y
	return
}

func canvasOverrun(r [4]float64, w, h int) string {
	var parts []string
	if r[0] < -canvasTolerance {
		parts = append(parts, fmt.Sprintf("%.0fpx off left", -r[0]))
	}
	if r[1] < -canvasTolerance {
		parts = append(parts, fmt.Sprintf("%.0fpx off top", -r[1]))
	}
	if over := r[0] + r[2] - float64(w); over > canvasTolerance {
		parts = append(parts, fmt.Sprintf("%.0fpx off right", over))
	}
	if over := r[1] + r[3] - float64(h); over > canvasTolerance {
		parts = append(parts, fmt.Sprintf("%.0fpx off bottom", over))
	}
	return strings.Join(parts, ", ")
}

func maxDelta(a, b [4]float64) float64 {
	var m float64
	for i := 0; i < 4; i++ {
		if d := absF(a[i] - b[i]); d > m {
			m = d
		}
	}
	return m
}

func reportBoxes(spec *Spec, table *SelTable, scenes []SceneFrames) map[string]map[string][][4]float64 {
	out := map[string]map[string][][4]float64{}
	for _, sel := range spec.ReportBoxes {
		perScene := map[string][][4]float64{}
		for _, sf := range scenes {
			if len(sf.Frames) == 0 {
				continue
			}
			for _, e := range sf.Frames[0] {
				if hasSel(&e, table.idx[sel]) {
					perScene[sf.Name] = append(perScene[sf.Name], e.Rect)
				}
			}
		}
		out[sel] = perScene
	}
	return out
}

func sortedPair(a, b string) []string {
	if a > b {
		a, b = b, a
	}
	return []string{a, b}
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func absF(a float64) float64 {
	if a < 0 {
		return -a
	}
	return a
}
