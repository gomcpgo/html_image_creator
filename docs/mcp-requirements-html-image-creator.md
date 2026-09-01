# Requirements: html_image_creator render_frames updates

For an implementation session in `/Users/prasanth/MyProjects/mcp/gomcpgo/html_image_creator`. Two features, independent of each other; both motivated by video 20 (Floyd-Warshall), whose full defect log is `videos/20-floyd-warshall/observations.md` in the algorithms repo. Relevant code: `pkg/frames/rules.go` (rule evaluation), `pkg/frames/types.go` (spec shapes), `pkg/frames/engine.go` (render/export loop), `pkg/frames/rules_test.go` (test patterns to follow), `docs/render-frames-spec.md` (user-facing spec doc, must be updated for both features).

## Feature A: static template elements as obstacles for annotations

### Problem
`no_overlap` checks the annotation selectors in `among` only against each other, and `keep_out` protects declared zones only. Nothing checks an annotation against the template's OWN static elements (map nodes, roads, edge-weight tags, chart cells). On video 20, three real defects reached the eye pass with a fully green report: a "start" tag sitting directly on a node, a "two cases" tag on a letter dot, and a mini-diagram dot on a map node. All three would have been violations at the validate_only gate if static elements were checkable obstacles.

### Requirement
Add an optional `obstacles` field (list of CSS selectors) to the `no_overlap` rule:

```json
{"type": "no_overlap", "among": [".tag", ".kalam"], "obstacles": ["#nodes circle", ".ewtag", ".cell"], "allow": [[".tag", ".ewtag"]]}
```

Behavior:
- Every element matched by `among` is additionally checked against every element matched by `obstacles`. Obstacle-vs-obstacle pairs are NOT checked (the template's own layout is by design).
- `allow` pairs apply to annotation-vs-obstacle pairs the same way they do today (and the existing discipline holds: an allow pair must be proven to fire on nothing before it stays in a spec).
- Violations report with the existing `no_overlap` type and name both selectors, so downstream tooling needs no change. If a distinct subtype is cheaper to read in reports, `no_overlap_obstacle` is acceptable; pick one and document it.
- Rule coverage applies: an `obstacles` selector that matches nothing in a scene where the rule is active is a coverage error, same as `among` selectors today.
- Measurement uses the same ink-based boxes as the rest of the engine.

### SVG obstacles (the known hard part)
The engine cannot measure SVG children (open request 3 in `mcp-changes.md`; lesson 42). Video 20's map nodes are SVG circles, so the exact defects that motivated this feature involve unmeasurable elements. Decide one of:
1. (Preferred) Minimal SVG geometry support for obstacles only: when an obstacle selector matches an SVG `circle`, `rect`, or `line`, derive its box from the geometry attributes (`cx/cy/r`, `x/y/width/height`, `x1/y1/x2/y2`) plus the layer's offset, without solving general SVG measurement. This covers nodes, roads and rings, which is the whole motivating class.
2. (Fallback) Document that obstacles must be positioned HTML elements, and that SVG-drawn boards need invisible positioned div proxies over each node (generator-emitted). If this route is taken, say so explicitly in `docs/render-frames-spec.md` so generators plan for it.

### Acceptance
- A spec with an annotation overlapping a div obstacle fails validate_only; moving it clears the run.
- An `allow` pair suppresses exactly its named annotation/obstacle pair.
- A dead obstacle selector produces a coverage error.
- If option 1 is taken: an annotation overlapping an SVG circle obstacle fails; one not overlapping passes.
- `docs/render-frames-spec.md` documents the field, the coverage rule, and the SVG decision.

## Feature B: partial re-export (per-scene / per-frame)

### Problem
`render_frames` always exports every frame. On video 20, two one-line coordinate fixes each forced a full 136-frame re-export (plus a Camtasia rebuild). The export is the slow part; validation is seconds.

### Requirement
Add optional filter fields to the render_frames request:

```json
{"spec_file": "...", "output_dir": "...", "export_scenes": ["scene03"], "export_frames": ["scene14-frame08"]}
```

Behavior:
- **Validation always runs over the FULL spec**, filters or not. Only PNG writing is filtered. A narrowed export must never narrow the gate.
- `export_scenes` selects whole scenes by name; `export_frames` selects individual frames by `sceneNN-frameNN`. Either alone or both together (union). Absent = export everything (today's behavior, unchanged).
- A filter entry that matches no scene/frame in the spec is an error naming the entry (same philosophy as coverage errors: a filter that selects nothing is a typo).
- The report states what was exported vs what was validated (e.g. `validated 136 frames, exported 1`), so a build log can never claim a full export that did not happen.
- `validate_only: true` with filters present: filters are ignored (nothing is exported anyway); not an error.

### Acceptance
- Filtered run writes only the named PNGs, leaves the rest of `output_dir` untouched, and still reports violations from unfiltered scenes.
- Unknown scene/frame name in a filter fails with a message naming it.
- Report distinguishes validated count from exported count.
- `docs/render-frames-spec.md` documents both fields and the validation-is-always-full rule.

## Out of scope here (already filed in mcp-changes.md)
Container-box exemptions (item 1), `no_overflow` (2), general SVG validation (3), per-frame `report_boxes` (4), `min_font` (5), spec hygiene and the `max_lines` null crash (6, 7), delta errors naming their frame (14), the mixed-font `max_lines` false positive (15), `free_regions` (16). Feature A touches item 3's territory; none of the rest blocks these two features.
