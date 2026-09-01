package frames

import (
	"path/filepath"
	"strings"
	"testing"
)

// helper: element at rect matching the given selector indices
func el(path, parent string, x, y, w, h float64, touched bool, sels ...int) Elem {
	return Elem{Path: path, Parent: parent, Rect: [4]float64{x, y, w, h}, Touched: touched, Sels: sels}
}

func specWith(rules ...Rule) *Spec {
	return &Spec{Width: 1920, Height: 1080, Validations: rules,
		Scenes: []Scene{{Name: "s", BaseHTML: "x"}}}
}

func violationsOf(rep *Report, rule string) []Violation {
	var out []Violation
	for _, sc := range rep.Scenes {
		for _, v := range sc.Violations {
			if v.Rule == rule {
				out = append(out, v)
			}
		}
	}
	return out
}

func TestInCanvas(t *testing.T) {
	spec := specWith(Rule{Type: "in_canvas", Selectors: []string{".card"}})
	table := BuildSelTable(spec)
	frames := [][]Elem{{
		el("#ok", "body", 10, 10, 100, 100, false, 0),
		el("#low", "body", 100, 1000, 200, 91, false, 0), // 11px off bottom
	}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	vs := violationsOf(rep, "in_canvas")
	if len(vs) != 1 || vs[0].Elements[0] != "#low" || !strings.Contains(vs[0].Detail, "11px off bottom") {
		t.Fatalf("got %+v", vs)
	}
}

func TestNoOverlapWithAllowAndAncestor(t *testing.T) {
	// sel 0 = .ann (among), sel 1 = .note, sel 2 = .dimmap (allow pair)
	spec := specWith(Rule{Type: "no_overlap", Among: []string{".ann"},
		Allow: [][]string{{".note", ".dimmap"}}})
	table := BuildSelTable(spec)
	iAnn, iNote, iDim := table.idx[".ann"], table.idx[".note"], table.idx[".dimmap"]

	frames := [][]Elem{{
		el("#a", "body", 0, 0, 100, 100, false, iAnn, iNote),
		el("#b", "body", 50, 50, 100, 100, false, iAnn, iNote), // overlaps #a -> violation
		el("#map", "body", 0, 0, 500, 500, false, iAnn, iDim),  // overlaps everything; allowed only vs .note
		el("#child", "#a", 10, 10, 20, 20, false, iAnn),        // inside #a: ancestor skip; overlaps #map -> violation (not a .note)
	}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	vs := violationsOf(rep, "no_overlap")
	if len(vs) != 2 {
		t.Fatalf("expected 2 violations (a-b, child-map), got %+v", vs)
	}
	for _, v := range vs {
		if v.Elements[0] == "#a" && v.Elements[1] == "#b" {
			if !strings.Contains(v.Detail, "50x50px") {
				t.Fatalf("bad detail %q", v.Detail)
			}
		} else if !(v.Elements[0] == "#child" && v.Elements[1] == "#map") {
			t.Fatalf("unexpected pair %+v", v.Elements)
		}
	}
}

func TestNoOverlapObstacles(t *testing.T) {
	// sel: .tag (among), .node + .ewtag (obstacles), allow .tag vs .ewtag
	spec := specWith(Rule{Type: "no_overlap", Among: []string{".tag"},
		Obstacles: []string{".node", ".ewtag"},
		Allow:     [][]string{{".tag", ".ewtag"}}})
	table := BuildSelTable(spec)
	iTag, iNode, iEw := table.idx[".tag"], table.idx[".node"], table.idx[".ewtag"]

	frames := [][]Elem{{
		el("#start", "body", 180, 180, 80, 40, false, iTag),  // on nodeA -> violation
		el("#clear", "body", 500, 700, 80, 40, false, iTag),  // clear of everything
		el("#weight", "body", 605, 405, 40, 20, false, iTag), // on ewtag -> allowed
		el("#nodeA", "#map", 170, 170, 60, 60, false, iNode),
		el("#nodeB", "#map", 140, 120, 60, 60, false, iNode), // overlaps #nodeA only: not checked
		el("#ew1", "#map", 600, 400, 50, 30, false, iEw),
	}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	vs := violationsOf(rep, "no_overlap")
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation (start-nodeA), got %+v", vs)
	}
	if vs[0].Elements[0] != "#nodeA" || vs[0].Elements[1] != "#start" {
		t.Fatalf("unexpected pair %+v", vs[0].Elements)
	}
	if len(rep.CoverageErrors) != 0 {
		t.Fatalf("unexpected coverage errors: %+v", rep.CoverageErrors)
	}
}

func TestNoOverlapObstacleAncestorSkip(t *testing.T) {
	// An annotation appended inside an obstacle container must not collide with it.
	spec := specWith(Rule{Type: "no_overlap", Among: []string{".tag"},
		Obstacles: []string{".zone"}})
	table := BuildSelTable(spec)
	iTag, iZone := table.idx[".tag"], table.idx[".zone"]
	frames := [][]Elem{{
		el("#zone", "body", 0, 0, 500, 500, false, iZone),
		el("#inside", "#zone", 10, 10, 80, 40, false, iTag),
	}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	if vs := violationsOf(rep, "no_overlap"); len(vs) != 0 {
		t.Fatalf("child vs ancestor obstacle flagged: %+v", vs)
	}
}

func TestDeadObstacleSelectorIsCoverageError(t *testing.T) {
	spec := specWith(Rule{Type: "no_overlap", Among: []string{".tag"},
		Obstacles: []string{".typo-never-matches"}})
	table := BuildSelTable(spec)
	frames := [][]Elem{{el("#a", "body", 0, 0, 10, 10, false, table.idx[".tag"])}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	if len(rep.CoverageErrors) != 1 || rep.Status != "violations_found" {
		t.Fatalf("got %+v status %s", rep.CoverageErrors, rep.Status)
	}
}

func TestNoOverlapTouchingBoxesPass(t *testing.T) {
	spec := specWith(Rule{Type: "no_overlap", Among: []string{".ann"}})
	table := BuildSelTable(spec)
	frames := [][]Elem{{
		el("#a", "body", 0, 0, 100, 100, false, 0),
		el("#b", "body", 100, 0, 100, 100, false, 0),  // edge-adjacent, no intersection
		el("#c", "body", 0, 99.5, 100, 100, false, 0), // 0.5px graze: within threshold
	}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	if vs := violationsOf(rep, "no_overlap"); len(vs) != 0 {
		t.Fatalf("expected none, got %+v", vs)
	}
}

func TestInkPreferredForOverlap(t *testing.T) {
	spec := specWith(Rule{Type: "no_overlap", Among: []string{".ann"}})
	table := BuildSelTable(spec)
	// centered text: band-wide layout box, narrow ink -> no overlap with the chip
	kalam := el("#note", "body", 0, 900, 1330, 80, false, 0)
	kalam.Ink = &[4]float64{400, 905, 500, 60}
	chipEl := el("#chip", "body", 1200, 900, 130, 60, false, 0)
	frames := [][]Elem{{kalam, chipEl}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	if vs := violationsOf(rep, "no_overlap"); len(vs) != 0 {
		t.Fatalf("ink should not overlap chip: %+v", vs)
	}
	// ink actually reaching the chip is still caught
	kalam.Ink = &[4]float64{400, 905, 850, 60}
	rep = Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: [][]Elem{{kalam, chipEl}}}})
	if vs := violationsOf(rep, "no_overlap"); len(vs) != 1 {
		t.Fatalf("expected ink overlap, got %+v", vs)
	}
}

func TestKeepOut(t *testing.T) {
	spec := specWith(Rule{Type: "keep_out", Selector: ".chip", Zones: []string{".rail-row"}})
	table := BuildSelTable(spec)
	iChip, iRow := table.idx[".chip"], table.idx[".rail-row"]
	frames := [][]Elem{{
		el("#chip1", "body", 1400, 360, 200, 50, false, iChip), // on row A
		el("#rowA", "#rail", 1390, 350, 470, 96, false, iRow),
		el("#chip2", "body", 100, 100, 200, 50, false, iChip), // clear
	}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	vs := violationsOf(rep, "keep_out")
	if len(vs) != 1 || vs[0].Elements[0] != "#chip1" {
		t.Fatalf("got %+v", vs)
	}
}

func TestMaxLines(t *testing.T) {
	spec := specWith(Rule{Type: "max_lines", Selector: ".huge", Lines: 1})
	table := BuildSelTable(spec)
	e := el("#punch", "body", 0, 0, 600, 260, false, 0)
	e.Lines = 2 // orphaned wrap
	frames := [][]Elem{{e}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	vs := violationsOf(rep, "max_lines")
	if len(vs) != 1 || !strings.Contains(vs[0].Detail, "2 lines") {
		t.Fatalf("got %+v", vs)
	}
}

func TestUntouchedMoved(t *testing.T) {
	spec := specWith() // implicit rule only
	table := BuildSelTable(spec)
	frames := [][]Elem{
		{el("#pin", "body", 100, 100, 50, 50, false)},
		{el("#pin", "body", 100, 103, 50, 50, false)}, // drifted 3px, untouched
	}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	vs := violationsOf(rep, "moved")
	if len(vs) != 1 || vs[0].FirstFrame != 2 {
		t.Fatalf("got %+v", vs)
	}

	// same drift but touched -> fine
	frames[1][0].Touched = true
	rep = Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	if vs := violationsOf(rep, "moved"); len(vs) != 0 {
		t.Fatalf("touched element flagged: %+v", vs)
	}
}

func TestConsecutiveDedupe(t *testing.T) {
	spec := specWith(Rule{Type: "no_overlap", Among: []string{".ann"}})
	table := BuildSelTable(spec)
	overlap := func(touched bool) []Elem {
		return []Elem{
			el("#a", "body", 0, 0, 100, 100, touched, 0),
			el("#b", "body", 50, 50, 100, 100, touched, 0),
		}
	}
	clear := []Elem{
		el("#a", "body", 0, 0, 100, 100, true, 0),
		el("#b", "body", 500, 500, 100, 100, true, 0),
	}
	// frames 1-3 overlap, frame 4 clear, frame 5 overlaps again
	frames := [][]Elem{overlap(false), overlap(true), overlap(true), clear, overlap(true)}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	vs := violationsOf(rep, "no_overlap")
	if len(vs) != 2 {
		t.Fatalf("expected 2 deduped entries, got %+v", vs)
	}
	if vs[0].FirstFrame != 1 || vs[0].LastFrame != 3 || vs[1].FirstFrame != 5 || vs[1].LastFrame != 5 {
		t.Fatalf("bad ranges: %+v", vs)
	}
}

func TestCoverageError(t *testing.T) {
	spec := specWith(Rule{Type: "in_canvas", Selectors: []string{".typo-never-matches"}})
	table := BuildSelTable(spec)
	frames := [][]Elem{{el("#x", "body", 0, 0, 10, 10, false)}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	if len(rep.CoverageErrors) != 1 || rep.Status != "violations_found" {
		t.Fatalf("got %+v status %s", rep.CoverageErrors, rep.Status)
	}
}

func TestAllowSelectorsNotCoverageWatched(t *testing.T) {
	spec := specWith(Rule{Type: "no_overlap", Among: []string{".ann"},
		Allow: [][]string{{".never-a", ".never-b"}}})
	table := BuildSelTable(spec)
	frames := [][]Elem{{el("#a", "body", 0, 0, 10, 10, false, table.idx[".ann"])}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	if len(rep.CoverageErrors) != 0 {
		t.Fatalf("allow selectors should not be coverage-watched: %+v", rep.CoverageErrors)
	}
}

func TestReportBoxes(t *testing.T) {
	spec := specWith()
	spec.ReportBoxes = []string{"#badge"}
	table := BuildSelTable(spec)
	frames := [][]Elem{{el("#badge", "body", 1710, 40, 150, 64, false, table.idx["#badge"])}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	boxes := rep.Boxes["#badge"]["s"]
	if len(boxes) != 1 || boxes[0] != [4]float64{1710, 40, 150, 64} {
		t.Fatalf("got %+v", rep.Boxes)
	}
}

func TestLineSelIdxIsNeverNil(t *testing.T) {
	// A nil slice marshals to JSON null, and the in-page JS calls .includes on it.
	spec := specWith(Rule{Type: "in_canvas", Selectors: []string{"#a"}})
	if idx := BuildSelTable(spec).LineSelIdx(spec); idx == nil {
		t.Fatal("LineSelIdx returned nil for a spec with no max_lines rule")
	}
}

func TestBrokenImage(t *testing.T) {
	spec := specWith()
	table := BuildSelTable(spec)
	img := el("#logo", "body", 10, 10, 24, 24, false)
	img.Broken = "receipts/missing.jpg"
	frames := [][]Elem{{img, el("#ok", "body", 0, 0, 100, 100, false)}}
	rep := Evaluate(spec, table, []SceneFrames{{Name: "s", Frames: frames}})
	vs := violationsOf(rep, "broken_image")
	if len(vs) != 1 || vs[0].Elements[0] != "#logo" || !strings.Contains(vs[0].Detail, "receipts/missing.jpg") {
		t.Fatalf("got %+v", vs)
	}
	if rep.Status != "violations_found" {
		t.Fatalf("broken image did not fail the run: %s", rep.Status)
	}
}

func TestResolveExportFilter(t *testing.T) {
	spec := &Spec{Scenes: []Scene{
		{Name: "scene01", BaseHTML: "x", Deltas: [][]Op{{{Op: "remove", Selector: "#a"}}, {{Op: "remove", Selector: "#b"}}}}, // 3 frames
		{Name: "scene02", BaseHTML: "x"}, // 1 frame
	}}

	if f, err := resolveExportFilter(spec, nil, nil); err != nil || f != nil {
		t.Fatalf("no filters should resolve to nil, got %v %v", f, err)
	}

	f, err := resolveExportFilter(spec, []string{"scene02"}, []string{"scene01-frame03"})
	if err != nil {
		t.Fatal(err)
	}
	if !f["scene02"][1] || !f["scene01"][3] || len(f["scene01"]) != 1 {
		t.Fatalf("bad filter %v", f)
	}

	for _, bad := range [][2][]string{
		{{"scene99"}, nil},         // unknown scene
		{nil, {"scene99-frame01"}}, // unknown scene in frame entry
		{nil, {"scene01-frame04"}}, // frame out of range
		{nil, {"scene01"}},         // not <scene>-frameNN
		{nil, {"scene01-frameXY"}}, // non-numeric frame
	} {
		if _, err := resolveExportFilter(spec, bad[0], bad[1]); err == nil {
			t.Fatalf("filter %v accepted", bad)
		}
	}

	// A dead entry must name itself in the error.
	_, err = resolveExportFilter(spec, []string{"scene99"}, nil)
	if err == nil || !strings.Contains(err.Error(), "scene99") {
		t.Fatalf("error does not name the entry: %v", err)
	}
}

func TestTotalsExported(t *testing.T) {
	spec := specWith()
	table := BuildSelTable(spec)
	scenes := []SceneFrames{
		{Name: "s1", Frames: [][]Elem{{el("#a", "body", 0, 0, 10, 10, false)}, {el("#a", "body", 0, 0, 10, 10, true)}}, Exported: 1},
		{Name: "s2", Frames: [][]Elem{{el("#b", "body", 0, 0, 10, 10, false)}}, Exported: 0},
	}
	rep := Evaluate(spec, table, scenes)
	if rep.Totals.Frames != 3 || rep.Totals.Exported != 1 {
		t.Fatalf("totals %+v", rep.Totals)
	}
}

func TestValidateSpec(t *testing.T) {
	good := &Spec{Width: 1920, Height: 1080, OutputDir: "/tmp/x",
		Scenes: []Scene{{Name: "s01", BaseHTML: "<div/>",
			Deltas: [][]Op{{{Op: "set_class", Selector: "#a", Class: "b"}}}}}}
	if err := ValidateSpec(good, false); err != nil {
		t.Fatalf("good spec rejected: %v", err)
	}
	bad := *good
	bad.Scenes = []Scene{{Name: "s01", BaseHTML: "<div/>", Deltas: [][]Op{{}}}}
	if err := ValidateSpec(&bad, false); err == nil {
		t.Fatal("empty delta accepted")
	}
	bad.Scenes = []Scene{{Name: "s01", BaseHTML: "<div/>",
		Deltas: [][]Op{{{Op: "teleport", Selector: "#a"}}}}}
	if err := ValidateSpec(&bad, false); err == nil {
		t.Fatal("unknown op accepted")
	}
	noOut := *good
	noOut.OutputDir = ""
	if err := ValidateSpec(&noOut, false); err == nil {
		t.Fatal("missing output_dir accepted")
	}
	if err := ValidateSpec(&noOut, true); err != nil {
		t.Fatalf("validate_only should not need output_dir: %v", err)
	}
	noAssets := *good
	noAssets.AssetDir = filepath.Join(t.TempDir(), "nope")
	if err := ValidateSpec(&noAssets, false); err == nil {
		t.Fatal("missing asset_dir accepted")
	}
	withAssets := *good
	withAssets.AssetDir = t.TempDir()
	if err := ValidateSpec(&withAssets, false); err != nil {
		t.Fatalf("existing asset_dir rejected: %v", err)
	}
}
