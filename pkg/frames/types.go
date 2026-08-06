package frames

import "fmt"

// Spec is the render_frames spec file format (see docs/render-frames-spec.md).
type Spec struct {
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	OutputDir   string   `json:"output_dir"`
	Scenes      []Scene  `json:"scenes"`
	Validations []Rule   `json:"validations"`
	ReportBoxes []string `json:"report_boxes"`
}

// Scene is a base frame plus cumulative deltas; frame count = len(Deltas)+1.
type Scene struct {
	Name     string `json:"name"`
	BaseHTML string `json:"base_html"`
	Deltas   [][]Op `json:"deltas"`
}

// Op is a single DOM mutation within a delta.
type Op struct {
	Op       string `json:"op"` // set_class | set_html | append_html | remove
	Selector string `json:"selector"`
	Class    string `json:"class,omitempty"`
	HTML     string `json:"html,omitempty"`
}

// Rule is one validation rule, discriminated by Type.
type Rule struct {
	Type      string     `json:"type"`                // in_canvas | no_overlap | keep_out | max_lines
	Selectors []string   `json:"selectors,omitempty"` // in_canvas
	Among     []string   `json:"among,omitempty"`     // no_overlap
	Allow     [][]string `json:"allow,omitempty"`     // no_overlap: pairs of selectors
	Selector  string     `json:"selector,omitempty"`  // keep_out, max_lines
	Zones     []string   `json:"zones,omitempty"`     // keep_out
	Lines     int        `json:"lines,omitempty"`     // max_lines
}

// Elem is one element's measurement in one frame, as returned by the in-page JS.
type Elem struct {
	Path    string     `json:"path"`
	Parent  string     `json:"parent"`
	Rect    [4]float64 `json:"rect"` // layout box: x, y, w, h in CSS px
	// Ink is the visual footprint. For elements with a background/border (and
	// all SVG elements) it equals Rect; for bare text it is the union of the
	// text line boxes — a centered headline's layout box spans its whole band,
	// its ink does not. Nil means "use Rect".
	Ink     *[4]float64 `json:"ink,omitempty"`
	Touched bool        `json:"touched"`
	Sels    []int       `json:"sels"` // indices into the selector table
	Lines   int         `json:"lines"`
}

// VBox is the element's visual footprint, used by geometric rules.
func (e *Elem) VBox() [4]float64 {
	if e.Ink != nil {
		return *e.Ink
	}
	return e.Rect
}

// SceneFrames is one scene's collected measurements, input to Evaluate.
type SceneFrames struct {
	Name     string
	Frames   [][]Elem
	Exported int
}

// Violation is one reported defect, deduped across consecutive frames.
type Violation struct {
	Rule       string   `json:"rule"`
	FirstFrame int      `json:"first_frame"`
	LastFrame  int      `json:"last_frame"`
	Elements   []string `json:"elements"`
	Detail     string   `json:"detail"`
}

// SceneResult is the per-scene section of the report.
type SceneResult struct {
	Name       string      `json:"name"`
	Frames     int         `json:"frames"`
	Exported   int         `json:"exported"`
	Violations []Violation `json:"violations"`
}

// Report is the tool's full result.
type Report struct {
	Status string `json:"status"` // succeeded | violations_found | spec_error
	Totals struct {
		Scenes     int `json:"scenes"`
		Frames     int `json:"frames"`
		Violations int `json:"violations"`
	} `json:"totals"`
	Scenes         []SceneResult                       `json:"scenes,omitempty"`
	CoverageErrors []string                            `json:"coverage_errors,omitempty"`
	Boxes          map[string]map[string][][4]float64  `json:"boxes,omitempty"`
	Error          string                              `json:"error,omitempty"`
}

// ValidateSpec checks structural validity before any rendering.
func ValidateSpec(spec *Spec, validateOnly bool) error {
	if spec.Width <= 0 || spec.Height <= 0 {
		return fmt.Errorf("width and height must be positive")
	}
	if !validateOnly && spec.OutputDir == "" {
		return fmt.Errorf("output_dir is required unless validate_only")
	}
	if len(spec.Scenes) == 0 {
		return fmt.Errorf("scenes must not be empty")
	}
	names := map[string]bool{}
	for si, sc := range spec.Scenes {
		if sc.Name == "" {
			return fmt.Errorf("scene %d: name is required", si)
		}
		if names[sc.Name] {
			return fmt.Errorf("scene name %q is duplicated", sc.Name)
		}
		names[sc.Name] = true
		if sc.BaseHTML == "" {
			return fmt.Errorf("scene %q: base_html is required", sc.Name)
		}
		for di, delta := range sc.Deltas {
			if len(delta) == 0 {
				return fmt.Errorf("scene %q delta %d: empty delta (every frame changes something)", sc.Name, di)
			}
			for oi, op := range delta {
				if op.Selector == "" {
					return fmt.Errorf("scene %q delta %d op %d: selector is required", sc.Name, di, oi)
				}
				switch op.Op {
				case "set_class":
					if op.Class == "" {
						return fmt.Errorf("scene %q delta %d op %d: set_class needs class", sc.Name, di, oi)
					}
				case "set_html", "append_html":
					if op.HTML == "" {
						return fmt.Errorf("scene %q delta %d op %d: %s needs html", sc.Name, di, oi, op.Op)
					}
				case "remove":
				default:
					return fmt.Errorf("scene %q delta %d op %d: unknown op %q", sc.Name, di, oi, op.Op)
				}
			}
		}
	}
	for ri, r := range spec.Validations {
		switch r.Type {
		case "in_canvas":
			if len(r.Selectors) == 0 {
				return fmt.Errorf("validation %d: in_canvas needs selectors", ri)
			}
		case "no_overlap":
			if len(r.Among) == 0 {
				return fmt.Errorf("validation %d: no_overlap needs among", ri)
			}
			for _, p := range r.Allow {
				if len(p) != 2 {
					return fmt.Errorf("validation %d: allow entries must be [selA, selB] pairs", ri)
				}
			}
		case "keep_out":
			if r.Selector == "" || len(r.Zones) == 0 {
				return fmt.Errorf("validation %d: keep_out needs selector and zones", ri)
			}
		case "max_lines":
			if r.Selector == "" || r.Lines <= 0 {
				return fmt.Errorf("validation %d: max_lines needs selector and lines", ri)
			}
		default:
			return fmt.Errorf("validation %d: unknown rule type %q", ri, r.Type)
		}
	}
	return nil
}
