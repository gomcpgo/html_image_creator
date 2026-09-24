# render_frames — implementation plan

Companion to `render-frames-spec.md`. Scope: one new tool in
`html_image_creator`; existing tools untouched.

## Code layout

```
pkg/frames/
  types.go    // Spec, Scene, Op, Rule, Report structs (JSON tags)
  engine.go   // per-scene render loop: rod page, apply ops, measure, screenshot
  rules.go    // pure-Go rule evaluation over measured boxes (no Chrome) + report assembly
  measure.js  // (embedded string) collects {domPath, selectorMatches, rect, lineCount}
```

- `engine.go` reuses the pattern from `pkg/screenshot` (temp HTTP server per
  scene dir, rod launch, viewport 2x, CSS reset injection, fonts.ready wait).
  One browser for the whole run; one page per scene.
- Per frame the engine runs one `page.Eval` that (a) applies the delta's ops,
  (b) returns measurements for every element: a stable DOM-path key, its
  bounding rect, which rule selectors it matches, whether it was targeted by
  this delta, and line-box count for `max_lines` targets.
- `rules.go` is pure Go over those measurements → unit-testable without Chrome:
  in_canvas, pairwise no_overlap with allow-list, keep_out, max_lines,
  untouched-moved (epsilon 0.5px), consecutive-frame dedupe into
  first_frame/last_frame, rule-coverage accounting across the run.

## Wiring

- `pkg/handler/tools.go`: add `render_frames` (spec_file, validate_only).
- `pkg/handler/handler.go`: dispatch → read spec file → `frames.Run(spec,
  validateOnly)` → JSON report response. `spec_error` (bad JSON, op selector
  matching nothing, missing output_dir when exporting) returns status
  `spec_error` with the offending scene/frame/op.
- `cmd/main.go`: flags `-render-frames <spec_file>`, `-validate-only`, same
  `runTerminalCommand` path as the other flags. This is the fast edit-build-run
  loop: `./run.sh build && bin/html_image_creator -render-frames spec.json`.
- `run.sh`: `render-frames <spec_file> [--validate-only]` case.

## Tests

1. **Unit (go test, no Chrome):** rules.go — overlap math incl. allow pairs
   and 1×1px threshold, in_canvas tolerance, untouched-moved epsilon,
   consecutive-frame dedupe, coverage failure when a rule never matches.
2. **Engine smoke (terminal mode, synthetic):** tiny base HTML + 3 deltas
   exercising all 4 ops; assert PNG count/dimensions, a planted overlap is
   reported with the correct first_frame, and a planted untouched-move
   (append that reflows a sibling) is caught.
3. **Video-04 baseline (real corpus):** a scratchpad Python script wraps each
   of the 85 existing `build/*.html` files as a one-frame scene with the
   validations from build-issues.md (annotation/chip/card/pill overlap rules,
   in_canvas, rail keep_out). Expected: current frames pass clean — they carry
   the session's fixes.

   **Baseline outcome (2026-08-06):** 85 frames audited in ~17 s. Two
   false-positive classes surfaced and were fixed by tuning (ink measurement
   for bare text; 4px box / 10px ink-ink intrusion thresholds). Final result:
   **8 real defects in the signed-off frames** — offer pills occluding price
   tags (scene03 f04 t-AC, scene06 f08–09 t-BC/t-BE, scene07 f01–03 t-DF),
   confirmed visually (the "9+4 = 13" pill half-covers the D–F "8" tag).
   Cross-scene `report_boxes`: badge and graph pixel-pinned across all 85
   frames; rail pinned across the 74 frames where it is visible. The
   deliberate-regression step was not needed — the corpus itself contained
   live defects that visual QA had passed.

## Order of work

1. types.go + rules.go + unit tests (pure Go, fastest feedback).
2. engine.go + measure.js; handler/tools/main wiring; build.
3. Synthetic smoke test in terminal mode.
4. Video-04 baseline + regression runs; tune rule details the corpus exposes
   (this is where the format is allowed to evolve — nothing speculative).

## Follow-ups (separate, after the tool proves out on a real video)

- Pipeline guide + build.md template updates to retire the post/export flow.
- Slim per-video generator convention (emit spec JSON instead of 85 HTML files).
