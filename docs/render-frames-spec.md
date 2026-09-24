# render_frames — tool specification

Renders a flipbook of frames (base HTML + cumulative deltas per scene) in one
headless-Chrome session per scene, validates layout geometry against declarative
rules, and exports PNGs. Built for the video-frame pipeline in
`ProgrammingTutorials/algorithms`; replaces the per-frame post/export flow.

## Tool contract

```
render_frames(spec_file, validate_only?, export_scenes?, export_frames?)
```

- `spec_file` (string, required) — absolute path to a spec JSON file (below).
- `validate_only` (bool, default false) — apply deltas and run validations but
  write no PNGs. Sub-second per scene; this is the design-phase audit gate.
  With `false`, the same run also screenshots every frame (build-phase export).
- `export_scenes` (array of strings, optional) - scene names whose frames should be written as PNGs, e.g. `["scene03"]`.
- `export_frames` (array of strings, optional) - individual frames to write, named `<scene>-frameNN`, e.g. `["scene14-frame08"]`.

**Partial re-export.** The two filter fields narrow which PNGs are written (union of both; absent = export everything). **Validation always runs over the FULL spec** - a narrowed export never narrows the gate, so a filtered run still reports violations from unfiltered scenes. A filter entry that matches no scene or frame in the spec is a `spec_error` naming the entry (a filter that selects nothing is a typo, same philosophy as rule coverage). With `validate_only: true` the filters are ignored (nothing is exported anyway); that is not an error. The report's `totals` distinguish `frames` (validated, always the full spec) from `exported`, so a build log can never claim a full export that did not happen.

Terminal mode: `bin/html_image_creator -render-frames <spec_file> [-validate-only] [-export-scenes scene02,scene05] [-export-frames scene01-frame02]`.

## Spec file format

```jsonc
{
  "width": 1920,
  "height": 1080,
  "output_dir": "/abs/path/frames",     // required unless validate_only
  "asset_dir": "/abs/path/assets",      // optional: served as the page's web root
  "scenes": [
    {
      "name": "scene06",                // PNG prefix: scene06-frame01.png …
      "base_html": "<!DOCTYPE html>…",  // full HTML of the scene's FIRST frame
      "deltas": [                       // deltas[i] turns frame i+1 into frame i+2
        [ {"op": "…", …}, {"op": "…", …} ],
        [ …ops for next frame… ]
      ]
    }
  ],
  "validations": [ …rules, see below… ],
  "report_boxes": ["#badge", ".graph"]  // optional: measured boxes returned per scene
}
```

Frame count per scene = `len(deltas) + 1`. A scene with no deltas is a single
frame — this is also how standalone HTML files are validated (wrap each as a
one-frame scene).

Each scene renders in ONE live page: load base → measure/screenshot → apply
delta 1 → measure/screenshot → … Elements therefore keep pixel-identical
positions across frames unless an op moves them, and the engine verifies that
(see implicit rules).

## Assets (images, CSS, fonts)

Each scene is served over a throwaway `http://127.0.0.1:<port>` server. The
generated HTML is served from memory at `/__frame__.html`; when `asset_dir` is
set, that directory is served read-only as the rest of the web root. So a scene
addresses assets by **path relative to `asset_dir`**:

```html
<link rel="stylesheet" href="frames.css">   <!-- <asset_dir>/frames.css -->
<img src="receipts/receipt-01.jpg">         <!-- <asset_dir>/receipts/receipt-01.jpg -->
```

Nothing is copied and nothing is written into `asset_dir` — the tool only reads
it. One root per spec, shared by every scene. A missing or non-directory
`asset_dir` is a `spec_error`.

Sharing a stylesheet this way is usually the bigger win: scene HTML shrinks to
structure, and the CSS stops being duplicated (and JSON-escaped) into every
scene.

Without `asset_dir` there is no web root, so **only `data:` URIs resolve** —
`file://` URLs and absolute filesystem paths do not, and never will (the page
origin is http, not file). Data URIs keep working whether or not `asset_dir`
is set.

Layout is measured after `document.fonts.ready` **and** after every `<img>`
has finished loading (10s cap), so an image inserted by a delta is measured at
its real size rather than 0×0.

## Delta ops

Grounded in what `gen_frames.py` (video 04) actually does — nothing more.
`selector` is any CSS selector. An op whose selector matches **zero** elements
is a hard error (the run fails with `spec_error`); matching several applies to
all.

| op            | fields              | effect                                        |
|---------------|---------------------|-----------------------------------------------|
| `set_class`   | `selector`, `class` | replaces the element's entire class attribute |
| `set_html`    | `selector`, `html`  | replaces the element's innerHTML              |
| `append_html` | `selector`, `html`  | inserts `html` as last child of the element   |
| `remove`      | `selector`          | removes the element(s) — annotation retirement|

`append_html` / `set_html` handle both HTML and SVG containers (SVG insertion
uses the SVG namespace automatically). Templates should provide stable
container elements to append into (e.g. `<div id="ann">`, `<g id="layer-mid">`)
— that is a template convention, not a tool feature.

## Validation rules

`validations` is an array of rule objects, applied to **every frame of every
scene**. All coordinates are CSS px (canvas space), measured after fonts and
images have loaded (see Assets).

**Visual boxes, not layout boxes.** Geometric rules use each element's visual
footprint: for painted elements (background, border, box-shadow — and all SVG
elements) that is `getBoundingClientRect`; for bare text it is the union of
its text line boxes ("ink") — a centered headline's layout box spans its whole
band, its ink does not. Intrusion thresholds: pairs must intersect by more
than **4px in both axes** (slivers and grazes pass); when both elements are
bare-text ink the threshold is **10px**, because line boxes include font
leading that overstates vertical ink by single-digit px.

| type         | fields                                   | violation when                                                        |
|--------------|------------------------------------------|-----------------------------------------------------------------------|
| `in_canvas`  | `selectors: [..]`                        | any matched element's visual box extends past the canvas (0.5px tolerance) |
| `no_overlap` | `among: [..]`, `obstacles: [..]`, `allow: [[selA, selB]..]` | two `among` elements' visual boxes intersect past the threshold, or an `among` element intersects an `obstacles` element, and the pair matches no `allow` entry |
| `keep_out`   | `selector`, `zones: [..]`                | a matched element's visual box intrudes into any zone element's box past the threshold |
| `max_lines`  | `selector`, `lines: N`                   | the element's text renders more line boxes than `lines` (wrap/orphan) |

### no_overlap obstacles

`obstacles` (optional) lists the template's own static elements - map nodes, roads, edge-weight tags, chart cells - that annotations must avoid:

```json
{"type": "no_overlap", "among": [".tag", ".kalam"], "obstacles": ["#nodes circle", ".ewtag", ".cell"], "allow": [[".tag", ".ewtag"]]}
```

Every `among` element is checked against every `obstacles` element. Obstacle-vs-obstacle pairs are NOT checked (the template's own layout is by design). Violations report with the plain `no_overlap` type and name both element paths, exactly like among-pairs; `allow` pairs apply to annotation-vs-obstacle pairs the same way (and the same discipline holds: an allow pair must be proven to fire on nothing before it stays in a spec). Rule coverage applies: an `obstacles` selector that matches nothing across the run is a coverage error, same as `among`. An annotation nested inside an obstacle (e.g. appended into an obstacle container) is exempt from that pair, same as the existing ancestor skip.

**SVG obstacles work directly.** SVG children (`circle`, `rect`, `line`, `path`, ...) are measured with `getBoundingClientRect` like any other element, so a selector such as `#nodes circle` is a valid obstacle with no proxy divs needed. The box is the geometry's bounding box after transforms; stroke width is not included, which is a good match for node dots and rings. (This also means the older "SVG is invisible to `no_overlap`" lesson no longer holds for this engine.)

### Implicit rules (always on)

1. **Untouched elements must not move.** Any element whose box changes between
   consecutive frames without being matched by that delta's ops (or being a
   descendant/new child of a target) is a violation. Catches reflow, drift, and
   phantom-blank-line bugs structurally, with no rule authoring.
2. **Images must load.** An `<img>` the browser failed to fetch is reported as
   a `broken_image` violation naming its `src`. Geometric rules cannot catch
   this — a broken-icon box is nothing like the intended box — so a run with a
   missing image reports `violations_found` instead of silently exporting PNGs
   with holes in them.
3. **Rule coverage.** A validation rule whose selectors match zero elements
   across the entire run fails the run (`coverage_error`) — a checker that
   watches nothing must not pass. (Zero matches in an individual frame is fine;
   annotations come and go.)

## Result report

```jsonc
{
  "status": "succeeded" | "violations_found" | "spec_error",
  "totals": { "scenes": 14, "frames": 85, "exported": 85, "violations": 3 },
                             // frames = validated (always the full spec);
                             // exported < frames on a filtered or validate_only run
  "scenes": [
    {
      "name": "scene06",
      "frames": 12,
      "exported": 12,               // 0 when validate_only
      "violations": [
        {
          "rule": "no_overlap",
          "first_frame": 6,         // delta that introduced it
          "last_frame": 12,         // persists through (cumulative dedupe)
          "elements": ["#note-half", "#note-neg"],
          "detail": "boxes overlap 84x31px at (1010,150)"
        }
      ]
    }
  ],
  "boxes": { "#badge": { "scene01": [1710,40,150,64], … } }  // from report_boxes,
                                                             // measured on frame 1
}
```

Identical violations (same rule, same elements) on consecutive frames are
reported once with `first_frame`/`last_frame` — the first frame is the delta
that introduced the defect. Violations do not stop rendering or export; the
report lists them and `status` says `violations_found`.

`report_boxes` exists so cross-**scene** invariants (badge, graph geometry,
rail rows pinned across all scenes) can be diffed by the caller from one run's
output — deliberately not a rule type until a real video needs more.

## Non-goals (deliberate)

- No image diffing, no OCR, no font-size floors — the shrink test and visual
  spot-checks remain separate pipeline steps.
- No per-scene validation overrides, no conditional ops, no templating in the
  spec. The spec is generated by a per-video script; logic lives there.
- Existing post/chart tools are untouched.
