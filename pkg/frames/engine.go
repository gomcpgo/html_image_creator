package frames

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// measureJS is evaluated once per frame: applies the delta's ops, waits for
// fonts and images, then measures every element (identity path, parent, rect,
// which rule selectors it matches, whether an image failed to load, line count
// for max_lines targets).
const measureJS = `async (ops, selectors, lineSelIdx) => {
	const touched = new Set();
	const markTree = (el) => {
		touched.add(el);
		for (const c of el.querySelectorAll('*')) touched.add(c);
	};
	for (let oi = 0; oi < ops.length; oi++) {
		const op = ops[oi];
		const targets = Array.from(document.querySelectorAll(op.selector));
		if (targets.length === 0)
			return { opError: 'op ' + oi + ' (' + op.op + '): selector "' + op.selector + '" matched no elements' };
		for (const el of targets) {
			if (op.op === 'set_class') { el.setAttribute('class', op.class); markTree(el); }
			else if (op.op === 'set_html') { el.innerHTML = op.html; markTree(el); }
			else if (op.op === 'append_html') {
				const before = el.childElementCount;
				el.insertAdjacentHTML('beforeend', op.html);
				touched.add(el); // the container may legitimately grow; its other children may not move
				for (let k = before; k < el.children.length; k++) markTree(el.children[k]);
			}
			else if (op.op === 'remove') { el.remove(); }
			else return { opError: 'op ' + oi + ': unknown op "' + op.op + '"' };
		}
	}
	void document.body.offsetHeight; // force layout so new font loads start
	await document.fonts.ready;
	// Images inserted by a delta have not loaded yet, and an <img> without
	// explicit dimensions measures 0x0 until it does. Capped so one stalled
	// request cannot hang the run.
	const pending = Array.from(document.images).filter(im => !im.complete);
	if (pending.length) {
		await Promise.race([
			Promise.all(pending.map(im => new Promise(res => {
				im.addEventListener('load', res, { once: true });
				im.addEventListener('error', res, { once: true });
			}))),
			new Promise(res => setTimeout(res, 10000)),
		]);
	}
	const paths = new Map();
	const pathOf = (el) => {
		if (!el || el.nodeType !== 1) return '';
		if (paths.has(el)) return paths.get(el);
		let p;
		if (el.id) p = '#' + el.id;
		else if (!el.parentElement) p = el.tagName.toLowerCase();
		else {
			let idx = 1;
			for (let s = el.previousElementSibling; s; s = s.previousElementSibling)
				if (s.tagName === el.tagName) idx++;
			p = pathOf(el.parentElement) + '>' + el.tagName.toLowerCase() + ':' + idx;
		}
		paths.set(el, p);
		return p;
	};
	const out = [];
	for (const el of document.body.querySelectorAll('*')) {
		const tag = el.tagName.toLowerCase();
		if (tag === 'script' || tag === 'style' || tag === 'link') continue;
		const sels = [];
		for (let i = 0; i < selectors.length; i++) {
			try { if (el.matches(selectors[i])) sels.push(i); } catch (e) {}
		}
		const r = el.getBoundingClientRect();
		// Ink: for bare text (no background/border, not SVG) the visual
		// footprint is the union of its line boxes, not the layout box —
		// centered text otherwise reports band-wide phantom overlaps.
		let ink = null;
		if (!(el instanceof SVGElement)) {
			const cs = getComputedStyle(el);
			const painted = (cs.backgroundColor !== 'rgba(0, 0, 0, 0)' && cs.backgroundColor !== 'transparent')
				|| cs.backgroundImage !== 'none'
				|| parseFloat(cs.borderTopWidth) > 0 || parseFloat(cs.borderRightWidth) > 0
				|| parseFloat(cs.borderBottomWidth) > 0 || parseFloat(cs.borderLeftWidth) > 0
				|| cs.boxShadow !== 'none';
			if (!painted) {
				const range = document.createRange();
				range.selectNodeContents(el);
				let x1 = Infinity, y1 = Infinity, x2 = -Infinity, y2 = -Infinity, any = false;
				for (const rr of range.getClientRects()) {
					if (rr.width < 1 || rr.height < 1) continue;
					any = true;
					x1 = Math.min(x1, rr.x); y1 = Math.min(y1, rr.y);
					x2 = Math.max(x2, rr.x + rr.width); y2 = Math.max(y2, rr.y + rr.height);
				}
				ink = any ? [x1, y1, x2 - x1, y2 - y1] : [r.x, r.y, 0, 0];
			}
		}
		// A broken image reports a broken-icon box, nothing like the intended
		// one — a defect the geometric rules cannot see.
		let broken = '';
		if (el instanceof HTMLImageElement && (!el.complete || el.naturalWidth === 0))
			broken = (el.getAttribute('src') || '(no src)').slice(0, 120);
		let lines = 0;
		if (lineSelIdx && sels.some(i => lineSelIdx.includes(i))) {
			const range = document.createRange();
			range.selectNodeContents(el);
			const tops = [];
			for (const rr of range.getClientRects()) {
				if (rr.width < 1 || rr.height < 1) continue;
				if (!tops.some(t => Math.abs(t - rr.top) < 2)) tops.push(rr.top);
			}
			lines = tops.length;
		}
		out.push({ path: pathOf(el), parent: pathOf(el.parentElement),
			rect: [r.x, r.y, r.width, r.height], ink, broken, touched: touched.has(el), sels, lines });
	}
	return { elements: out };
}`

// frameHTMLName is where the generated scene HTML is served, at the web root
// alongside the asset files; unlikely to collide with a real asset.
const frameHTMLName = "__frame__.html"

type evalResult struct {
	OpError  string `json:"opError"`
	Elements []Elem `json:"elements"`
}

// Run renders every scene in the spec, evaluates validations, and (unless
// validateOnly) exports PNGs. Spec/content problems come back as a Report
// with status spec_error; only infrastructure failures return a Go error.
func Run(spec *Spec, validateOnly bool) (*Report, error) {
	if err := ValidateSpec(spec, validateOnly); err != nil {
		return specErrorReport(err), nil
	}
	if !validateOnly {
		if err := os.MkdirAll(spec.OutputDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output dir: %w", err)
		}
	}

	table := BuildSelTable(spec)
	lineSelIdx := table.LineSelIdx(spec)

	chromePath, _ := launcher.LookPath()
	l := launcher.New().Headless(true)
	if chromePath != "" {
		l = l.Bin(chromePath)
	}
	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch chrome: %w", err)
	}
	browser := rod.New().ControlURL(controlURL).Context(context.Background())
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("chrome not available: %w", err)
	}
	defer browser.MustClose()

	var collected []SceneFrames
	for i := range spec.Scenes {
		sf, serr, err := renderScene(browser, spec, &spec.Scenes[i], table, lineSelIdx, validateOnly)
		if err != nil {
			return nil, err
		}
		if serr != "" {
			return specErrorReport(fmt.Errorf("scene %q: %s", spec.Scenes[i].Name, serr)), nil
		}
		collected = append(collected, sf)
	}

	rep := Evaluate(spec, table, collected)
	return rep, nil
}

// renderScene runs one scene in one page. Returns (measurements, specError, infraError).
func renderScene(browser *rod.Browser, spec *Spec, scene *Scene, table *SelTable,
	lineSelIdx []int, validateOnly bool) (SceneFrames, string, error) {
	sf := SceneFrames{Name: scene.Name}

	html := injectCSSReset(scene.BaseHTML)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return sf, "", fmt.Errorf("failed to start temp server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	// The scene HTML is served from memory at a reserved name, so relative asset
	// paths resolve against asset_dir without anything being written into it.
	mux := http.NewServeMux()
	mux.HandleFunc("/"+frameHTMLName, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
	})
	if spec.AssetDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(spec.AssetDir)))
	}
	httpServer := &http.Server{Handler: mux}
	go httpServer.Serve(listener)
	defer httpServer.Close()

	page, err := browser.Page(proto.TargetCreateTarget{URL: fmt.Sprintf("http://127.0.0.1:%d/%s", port, frameHTMLName)})
	if err != nil {
		return sf, "", fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()
	page = page.Timeout(2 * time.Minute)

	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width: spec.Width, Height: spec.Height, DeviceScaleFactor: 2,
	}); err != nil {
		return sf, "", fmt.Errorf("failed to set viewport: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return sf, "", fmt.Errorf("failed to load page: %w", err)
	}
	if _, err := page.Eval(`() => document.fonts.ready`); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: fonts.ready failed for %s: %v\n", scene.Name, err)
	}

	totalFrames := len(scene.Deltas) + 1
	for f := 1; f <= totalFrames; f++ {
		ops := []Op{}
		if f > 1 {
			ops = scene.Deltas[f-2]
		}
		res, err := page.Eval(measureJS, ops, table.Sels, lineSelIdx)
		if err != nil {
			return sf, "", fmt.Errorf("scene %s frame %d: eval failed: %w", scene.Name, f, err)
		}
		var er evalResult
		raw, err := res.Value.MarshalJSON()
		if err != nil {
			return sf, "", fmt.Errorf("scene %s frame %d: bad eval result: %w", scene.Name, f, err)
		}
		if err := json.Unmarshal(raw, &er); err != nil {
			return sf, "", fmt.Errorf("scene %s frame %d: bad eval result: %w", scene.Name, f, err)
		}
		if er.OpError != "" {
			return sf, fmt.Sprintf("frame %d: %s", f, er.OpError), nil
		}
		sf.Frames = append(sf.Frames, er.Elements)

		if !validateOnly {
			shot, err := page.Screenshot(true, &proto.PageCaptureScreenshot{
				Format: proto.PageCaptureScreenshotFormatPng,
				Clip: &proto.PageViewport{
					X: 0, Y: 0,
					Width:  float64(spec.Width),
					Height: float64(spec.Height),
					Scale:  1,
				},
			})
			if err != nil {
				return sf, "", fmt.Errorf("scene %s frame %d: screenshot failed: %w", scene.Name, f, err)
			}
			out := filepath.Join(spec.OutputDir, fmt.Sprintf("%s-frame%02d.png", scene.Name, f))
			if err := os.WriteFile(out, shot, 0644); err != nil {
				return sf, "", fmt.Errorf("failed to write %s: %w", out, err)
			}
			sf.Exported++
		}
	}
	return sf, "", nil
}

func specErrorReport(err error) *Report {
	return &Report{Status: "spec_error", Error: err.Error()}
}

// injectCSSReset matches the behavior of pkg/screenshot for consistent output.
func injectCSSReset(htmlContent string) string {
	cssReset := `<style>html,body{margin:0;padding:0;overflow:hidden;}</style>`
	if idx := strings.Index(strings.ToLower(htmlContent), "</head>"); idx != -1 {
		return htmlContent[:idx] + cssReset + "\n" + htmlContent[idx:]
	}
	if idx := strings.Index(strings.ToLower(htmlContent), "<body"); idx != -1 {
		if endIdx := strings.Index(htmlContent[idx:], ">"); endIdx != -1 {
			insertPos := idx + endIdx + 1
			return htmlContent[:insertPos] + "\n" + cssReset + "\n" + htmlContent[insertPos:]
		}
	}
	return cssReset + "\n" + htmlContent
}
