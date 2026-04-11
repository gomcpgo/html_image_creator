package screenshot

import (
	"context"
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

// Screenshotter handles taking screenshots of HTML posts via headless Chrome
type Screenshotter struct {
	chromeTimeout time.Duration
	libsDir       string // shared directory for bundled JS libraries
}

// NewScreenshotter creates a new Screenshotter instance
func NewScreenshotter(libsDir string) *Screenshotter {
	return &Screenshotter{
		chromeTimeout: 30 * time.Second,
		libsDir:       libsDir,
	}
}

// TakeScreenshot renders an HTML post at exact dimensions and saves as PNG
func (s *Screenshotter) TakeScreenshot(postDir string, width, height int, outputPath string) error {
	// Read and prepare HTML with CSS reset
	htmlPath := filepath.Join(postDir, "index.html")
	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		return fmt.Errorf("failed to read HTML file: %w", err)
	}

	htmlContent := injectCSSReset(string(htmlBytes))

	// Write temp HTML file with CSS reset injected
	tmpHTMLPath := filepath.Join(postDir, "temp_screenshot.html")
	if err := os.WriteFile(tmpHTMLPath, []byte(htmlContent), 0644); err != nil {
		return fmt.Errorf("failed to write temp HTML file: %w", err)
	}
	defer os.Remove(tmpHTMLPath)

	// Start a temporary local HTTP server to serve the post directory and shared libs
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start temp server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	if s.libsDir != "" {
		mux.Handle("/libs/", http.StripPrefix("/libs/", http.FileServer(http.Dir(s.libsDir))))
	}
	mux.Handle("/", http.FileServer(http.Dir(postDir)))
	httpServer := &http.Server{Handler: mux}
	go httpServer.Serve(listener)
	defer httpServer.Close()

	// Launch headless Chrome
	ctx, cancel := context.WithTimeout(context.Background(), s.chromeTimeout)
	defer cancel()

	chromePath, _ := launcher.LookPath()

	var controlURL string
	if chromePath != "" {
		l := launcher.New().Bin(chromePath).Headless(true)
		controlURL = l.MustLaunch()
	} else {
		l := launcher.New().Headless(true)
		controlURL = l.MustLaunch()
	}

	browser := rod.New().ControlURL(controlURL).Context(ctx)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("chrome not available: %w", err)
	}
	defer browser.MustClose()

	// Create page and set viewport to exact canvas dimensions
	pageURL := fmt.Sprintf("http://127.0.0.1:%d/temp_screenshot.html", port)
	page, err := browser.Page(proto.TargetCreateTarget{URL: pageURL})
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}

	// Set exact viewport dimensions with 2x scale for high-res output
	err = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             width,
		Height:            height,
		DeviceScaleFactor: 2,
	})
	if err != nil {
		return fmt.Errorf("failed to set viewport: %w", err)
	}

	// Wait for page to fully load
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("failed to load page: %w", err)
	}

	// Wait for fonts to load
	_, err = page.Eval(`() => document.fonts.ready`)
	if err != nil {
		// Non-fatal: continue even if fonts.ready fails
		fmt.Printf("Warning: fonts.ready check failed: %v\n", err)
	}

	// Wait for chart rendering signal (document.title === 'ready')
	// After signal fires, wait 2s for Chart.js/D3 canvas animations to complete.
	// Chart.js default animation duration is 1000ms; 2s provides safe margin.
	// Times out after 5 seconds for non-chart posts or if signal is not set.
	waitPage := page.Timeout(5 * time.Second)
	_, err = waitPage.Eval(`() => new Promise((resolve) => {
		function onReady() { setTimeout(resolve, 2000); }
		if (document.title === 'ready') return onReady();
		const titleEl = document.querySelector('title');
		if (!titleEl) return resolve();
		const observer = new MutationObserver(() => {
			if (document.title === 'ready') { observer.disconnect(); onReady(); }
		});
		observer.observe(titleEl, { childList: true });
	})`)
	if err != nil {
		// Non-fatal: timeout means no chart signal, proceed with screenshot
	}

	// Take screenshot of the viewport
	screenshotData, err := page.Screenshot(true, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
		Clip: &proto.PageViewport{
			X:      0,
			Y:      0,
			Width:  float64(width),
			Height: float64(height),
			Scale:  1,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to take screenshot: %w", err)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write PNG to output path
	if err := os.WriteFile(outputPath, screenshotData, 0644); err != nil {
		return fmt.Errorf("failed to write screenshot: %w", err)
	}

	return nil
}

// injectCSSReset injects a CSS reset to ensure accurate viewport rendering
func injectCSSReset(htmlContent string) string {
	cssReset := `<style>html,body{margin:0;padding:0;overflow:hidden;}</style>`

	// Inject before </head> if present
	if idx := strings.Index(strings.ToLower(htmlContent), "</head>"); idx != -1 {
		return htmlContent[:idx] + cssReset + "\n" + htmlContent[idx:]
	}

	// Inject after <body> if present
	if idx := strings.Index(strings.ToLower(htmlContent), "<body"); idx != -1 {
		if endIdx := strings.Index(htmlContent[idx:], ">"); endIdx != -1 {
			insertPos := idx + endIdx + 1
			return htmlContent[:insertPos] + "\n" + cssReset + "\n" + htmlContent[insertPos:]
		}
	}

	// Fallback: prepend
	return cssReset + "\n" + htmlContent
}
