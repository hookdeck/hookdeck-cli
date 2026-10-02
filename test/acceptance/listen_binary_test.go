//go:build listen

package acceptance

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// legacyCLIVersion is the last release before binary delivery: it sends no
// X-Hookdeck-CLI-Capabilities header and only reads data_string.
const legacyCLIVersion = "3.0.3"

// legacyCLIChecksums pins the SHA-256 of each release tarball, from the
// release's published checksum files.
var legacyCLIChecksums = map[string]string{
	"darwin_amd64": "a2ccc50db7211cadb20d23f8025b69b1b8ba180d92cfb4c92d7fd74e907f73e8",
	"darwin_arm64": "b26f0e4a077c80cdde5cef8f71f189e27bcdf4cbc5792ce89f765f310f147764",
	"linux_amd64":  "73ecce58128efb8dce09657085f642bb470b618857ff4d53d400b585cf65d764",
	"linux_arm64":  "a98f6e46ca3d147d37af12cdf8a08651818097c20f521cf6ebc1131b9ef65bf7",
}

// downloadLegacyCLI fetches the released v3.0.3 binary for this platform,
// verifies it against the pinned checksum, and returns its path.
func downloadLegacyCLI(t *testing.T) string {
	t.Helper()
	platform := runtime.GOOS + "_" + runtime.GOARCH
	want, ok := legacyCLIChecksums[platform]
	require.True(t, ok, "no pinned v%s release for %s", legacyCLIVersion, platform)

	asset := fmt.Sprintf("hookdeck_%s_%s.tar.gz", legacyCLIVersion, platform)
	url := fmt.Sprintf("https://github.com/hookdeck/hookdeck-cli/releases/download/v%s/%s", legacyCLIVersion, asset)
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Get(url)
	require.NoError(t, err, "download %s", url)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "download %s", url)
	archive, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, want, fmt.Sprintf("%x", sha256.Sum256(archive)), "checksum mismatch for %s", asset)

	gz, err := gzip.NewReader(bytes.NewReader(archive))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		require.NoError(t, err, "hookdeck binary not found in %s", asset)
		if hdr.Name != "hookdeck" {
			continue
		}
		binary := filepath.Join(t.TempDir(), "hookdeck-"+legacyCLIVersion)
		f, err := os.OpenFile(binary, os.O_CREATE|os.O_WRONLY, 0o755)
		require.NoError(t, err)
		_, err = io.Copy(f, tr)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		return binary
	}
}

// ---------------------------------------------------------------------------
// Local app
// ---------------------------------------------------------------------------

// receivedRequest is one request as the local app saw it. Files holds what the
// app wrote to disk: the raw body for single-file requests, or every file part
// of a multipart form, keyed by form field name.
type receivedRequest struct {
	contentType  string
	eventID      string
	attemptCount int
	body         []byte
	files        map[string]string
}

// localApp stands in for the user's application: it saves incoming bodies to
// disk the way an upload endpoint would, using only the standard library.
type localApp struct {
	server   *httptest.Server
	port     string
	received chan receivedRequest

	mu sync.Mutex
	// status returns the HTTP status for the nth request (1-based).
	status func(n int) int
	count  int
}

func startLocalApp(t *testing.T) *localApp {
	t.Helper()
	app := &localApp{received: make(chan receivedRequest, 16), status: func(int) int { return http.StatusOK }}
	dir := t.TempDir()

	app.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.mu.Lock()
		app.count++
		n := app.count
		status := app.status(n)
		app.mu.Unlock()

		req := receivedRequest{
			contentType: r.Header.Get("Content-Type"),
			eventID:     r.Header.Get("X-Hookdeck-EventID"),
			files:       map[string]string{},
		}
		req.attemptCount, _ = strconv.Atoi(r.Header.Get("X-Hookdeck-Attempt-Count"))

		mediaType, _, _ := mime.ParseMediaType(req.contentType)
		if mediaType == "multipart/form-data" {
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				t.Errorf("local app: ParseMultipartForm: %v", err)
			} else {
				for field, headers := range r.MultipartForm.File {
					src, err := headers[0].Open()
					require.NoError(t, err)
					path := filepath.Join(dir, fmt.Sprintf("%d-%s-%s", n, field, filepath.Base(headers[0].Filename)))
					dst, err := os.Create(path)
					require.NoError(t, err)
					_, err = io.Copy(dst, src)
					require.NoError(t, err)
					require.NoError(t, dst.Close())
					require.NoError(t, src.Close())
					req.files[field] = path
				}
			}
		} else {
			body, _ := io.ReadAll(r.Body)
			req.body = body
			exts, _ := mime.ExtensionsByType(mediaType)
			ext := ".bin"
			if len(exts) > 0 {
				ext = exts[0]
			}
			path := filepath.Join(dir, fmt.Sprintf("%d-body%s", n, ext))
			require.NoError(t, os.WriteFile(path, body, 0o600))
			req.files["body"] = path
		}

		select {
		case app.received <- req:
		default:
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(app.server.Close)

	u, err := url.Parse(app.server.URL)
	require.NoError(t, err)
	app.port = u.Port()
	require.NotEmpty(t, app.port)
	return app
}

func (a *localApp) setStatus(status func(n int) int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = status
}

func (a *localApp) next(t *testing.T, within time.Duration, outputs ...*syncBuffer) receivedRequest {
	t.Helper()
	select {
	case r := <-a.received:
		return r
	case <-time.After(within):
		for _, o := range outputs {
			t.Logf("listen output: %s", o.String())
		}
		t.Fatalf("no request reached the local app within %s; if the event failed with CLI_BINARY_UNSUPPORTED, the server does not deliver binary bodies to this CLI", within)
		return receivedRequest{}
	}
}

func (a *localApp) expectNothing(t *testing.T, within time.Duration) {
	t.Helper()
	select {
	case r := <-a.received:
		t.Fatalf("expected no delivery, got %d bytes of %q", len(r.body), r.contentType)
	case <-time.After(within):
	}
}

// ---------------------------------------------------------------------------
// Fixtures: real media, generated deterministically
// ---------------------------------------------------------------------------

type fixture struct {
	name        string
	contentType string
	data        []byte
	// validate decodes the file the app saved, proving it is usable, not just
	// the same length.
	validate func(t *testing.T, data []byte)
}

func testImage() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: uint8((x ^ y) * 8), A: 255})
		}
	}
	return img
}

func pngFixture(t *testing.T) fixture {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, testImage()))
	return fixture{name: "picture.png", contentType: "image/png", data: buf.Bytes(), validate: func(t *testing.T, data []byte) {
		img, err := png.Decode(bytes.NewReader(data))
		require.NoError(t, err, "saved PNG must decode")
		assert.Equal(t, testImage().Bounds(), img.Bounds())
	}}
}

func jpegFixture(t *testing.T) fixture {
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, testImage(), &jpeg.Options{Quality: 90}))
	return fixture{name: "photo.jpg", contentType: "image/jpeg", data: buf.Bytes(), validate: func(t *testing.T, data []byte) {
		img, err := jpeg.Decode(bytes.NewReader(data))
		require.NoError(t, err, "saved JPEG must decode")
		assert.Equal(t, testImage().Bounds(), img.Bounds())
	}}
}

// wavFixture is 0.1s of a 440Hz tone as 16-bit PCM, which covers every byte
// value in its samples.
func wavFixture() fixture {
	const sampleRate, samples = 8000, 800
	var pcm bytes.Buffer
	for i := 0; i < samples; i++ {
		v := int16(math.Sin(2*math.Pi*440*float64(i)/sampleRate) * 32000)
		_ = binary.Write(&pcm, binary.LittleEndian, v)
	}
	var buf bytes.Buffer
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36+pcm.Len()))
	buf.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(sampleRate), uint32(sampleRate * 2), uint16(2), uint16(16)} {
		_ = binary.Write(&buf, binary.LittleEndian, v)
	}
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(pcm.Len()))
	buf.Write(pcm.Bytes())
	return fixture{name: "tone.wav", contentType: "audio/wav", data: buf.Bytes(), validate: func(t *testing.T, data []byte) {
		require.GreaterOrEqual(t, len(data), 44)
		assert.Equal(t, "RIFF", string(data[0:4]))
		assert.Equal(t, "WAVE", string(data[8:12]))
		assert.Equal(t, uint32(samples*2), binary.LittleEndian.Uint32(data[40:44]), "WAV data chunk length")
	}}
}

// pdfFixture is a minimal one-page PDF with the customary binary comment line.
func pdfFixture() fixture {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 72 72] >>",
	}
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return fixture{name: "doc.pdf", contentType: "application/pdf", data: buf.Bytes(), validate: func(t *testing.T, data []byte) {
		assert.True(t, bytes.HasPrefix(data, []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3")), "PDF header and binary marker")
		assert.True(t, bytes.HasSuffix(data, []byte("%%EOF\n")), "PDF trailer")
	}}
}

func allBytesFixture() fixture {
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	return fixture{name: "all-bytes.bin", contentType: "application/octet-stream", data: data, validate: func(*testing.T, []byte) {}}
}

// multipartUpload builds a form with a text field and one file part per
// fixture, the way a browser or SDK upload would.
func multipartUpload(t *testing.T, files ...fixture) (contentType string, body []byte) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.WriteField("caption", "binary delivery acceptance"))
	for i, f := range files {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file%d"; filename=%q`, i, f.name))
		h.Set("Content-Type", f.contentType)
		part, err := w.CreatePart(h)
		require.NoError(t, err)
		_, err = part.Write(f.data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return w.FormDataContentType(), buf.Bytes()
}

// assertSavedFile checks the file the app wrote is the file that was sent,
// byte for byte, and still decodes as its format.
func assertSavedFile(t *testing.T, path string, want fixture) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, sha256.Sum256(want.data), sha256.Sum256(got),
		"%s: saved file differs from the original (%d bytes sent, %d saved)", want.name, len(want.data), len(got))
	want.validate(t, got)
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type binaryListenSetup struct {
	cli        *CLIRunner
	sourceName string
	sourceURL  string
	connID     string
}

func newBinaryListenSetup(t *testing.T, prefix string) binaryListenSetup {
	t.Helper()
	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()
	sourceName := prefix + "-" + timestamp

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn,
		"gateway", "connection", "create",
		"--name", prefix+"-conn-"+timestamp,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", prefix+"-dst-"+timestamp,
		"--destination-type", "CLI",
		"--destination-cli-path", "/",
	))
	require.NotEmpty(t, conn.ID)
	t.Cleanup(func() { deleteConnection(t, cli, conn.ID) })

	var src Source
	require.NoError(t, cli.RunJSON(&src, "gateway", "source", "get", conn.Source.ID))
	require.NotEmpty(t, src.URL)
	return binaryListenSetup{cli: cli, sourceName: sourceName, sourceURL: src.URL, connID: conn.ID}
}

// waitForTunnel gives listen time to connect and fails fast if it exited.
func waitForTunnel(t *testing.T, done chan error, outputs ...*syncBuffer) {
	t.Helper()
	t.Log("Waiting 12 seconds for the tunnel to connect...")
	time.Sleep(12 * time.Second)
	select {
	case err := <-done:
		for _, o := range outputs {
			t.Logf("listen output: %s", o.String())
		}
		t.Fatalf("listen exited before forwarding anything: %v", err)
	default:
	}
}

// postRaw sends body to a source URL with the given Content-Type, unmodified.
func postRaw(t *testing.T, sourceURL, contentType string, body []byte) {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(sourceURL, contentType, bytes.NewReader(body))
	require.NoError(t, err, "POST to source URL failed")
	defer resp.Body.Close()
	require.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
		"POST to source URL returned %d", resp.StatusCode)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestListenForwardsBinaryFilesThatTheAppCanSave sends real files (images, a
// PDF, audio inside a multipart upload, and every byte value) through a source
// and `hookdeck listen`, and checks the local app can save each one back to a
// file identical to the original.
func TestListenForwardsBinaryFilesThatTheAppCanSave(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	s := newBinaryListenSetup(t, "test-bin")
	app := startLocalApp(t)
	_, stdout, stderr, done := startListenCapturingOutput(t, s.cli, "listen", app.port, s.sourceName, "--output", "compact")
	waitForTunnel(t, done, stdout, stderr)

	// Raw bodies: the content types ingestion stores as binary.
	for _, f := range []fixture{allBytesFixture(), pngFixture(t), jpegFixture(t), pdfFixture()} {
		f := f
		t.Run("raw "+f.contentType, func(t *testing.T) {
			postRaw(t, s.sourceURL, f.contentType, f.data)
			got := app.next(t, 45*time.Second, stdout, stderr)
			assert.Equal(t, f.contentType, got.contentType, "original Content-Type must be forwarded")
			assertSavedFile(t, got.files["body"], f)
		})
	}

	t.Run("multipart upload with picture and audio", func(t *testing.T) {
		files := []fixture{pngFixture(t), wavFixture(), allBytesFixture()}
		contentType, body := multipartUpload(t, files...)

		postRaw(t, s.sourceURL, contentType, body)
		got := app.next(t, 45*time.Second, stdout, stderr)

		assert.Equal(t, contentType, got.contentType, "boundary travels in the original Content-Type")
		require.Len(t, got.files, len(files), "the app should save every file part")
		for i, f := range files {
			assertSavedFile(t, got.files[fmt.Sprintf("file%d", i)], f)
		}
	})
}

// TestListenRetriesBinaryBodyByteExact fails the first delivery, retries the
// event, and checks the retried attempt carries the same bytes. Retries resolve
// the CLI session through a different path from first attempts.
func TestListenRetriesBinaryBodyByteExact(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	s := newBinaryListenSetup(t, "test-bin-retry")
	app := startLocalApp(t)
	app.setStatus(func(n int) int {
		if n == 1 {
			return http.StatusInternalServerError
		}
		return http.StatusOK
	})
	_, stdout, stderr, done := startListenCapturingOutput(t, s.cli, "listen", app.port, s.sourceName, "--output", "compact")
	waitForTunnel(t, done, stdout, stderr)

	f := pngFixture(t)
	postRaw(t, s.sourceURL, f.contentType, f.data)

	first := app.next(t, 45*time.Second, stdout, stderr)
	require.NotEmpty(t, first.eventID, "delivery should carry X-Hookdeck-EventID")
	assert.Equal(t, 1, first.attemptCount)
	assertSavedFile(t, first.files["body"], f)

	s.cli.RunExpectSuccess("gateway", "event", "retry", first.eventID)

	second := app.next(t, 45*time.Second, stdout, stderr)
	assert.Equal(t, first.eventID, second.eventID, "the retry should deliver the same event")
	assert.Equal(t, 2, second.attemptCount)
	assert.Equal(t, f.contentType, second.contentType)
	assertSavedFile(t, second.files["body"], f)
}

// TestListenBinaryWithOldAndNewCLIListening runs the released v3.0.3 CLI, which
// predates binary delivery, and this CLI on the same source at the same time,
// as separate CLI clients. Each session gets its own event: this CLI receives
// the exact bytes, while the old CLI's event fails closed (or, for multipart,
// is delivered as the lossy text it always got).
func TestListenBinaryWithOldAndNewCLIListening(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	legacyBinary := downloadLegacyCLI(t)

	s := newBinaryListenSetup(t, "test-bin-mixed")
	legacyCLI := newSeparateCLIClient(t, s.cli)
	newApp := startLocalApp(t)
	oldApp := startLocalApp(t)
	_, newOut, newErr, newDone := startListenCapturingOutput(t, s.cli, "listen", newApp.port, s.sourceName, "--output", "compact")
	_, oldOut, oldErr, oldDone := startListenBinaryCapturingOutput(t, legacyCLI, legacyBinary, "listen", oldApp.port, s.sourceName, "--output", "compact")
	waitForTunnel(t, newDone, newOut, newErr)
	waitForTunnel(t, oldDone, oldOut, oldErr)

	t.Run("raw image", func(t *testing.T) {
		f := pngFixture(t)
		postRaw(t, s.sourceURL, f.contentType, f.data)

		got := newApp.next(t, 45*time.Second, newOut, newErr)
		assertSavedFile(t, got.files["body"], f)
		oldApp.expectNothing(t, 20*time.Second)

		assertEventFailedWithCode(t, s, "CLI_BINARY_UNSUPPORTED")
	})

	t.Run("multipart upload", func(t *testing.T) {
		files := []fixture{pngFixture(t), wavFixture()}
		contentType, body := multipartUpload(t, files...)
		postRaw(t, s.sourceURL, contentType, body)

		got := newApp.next(t, 45*time.Second, newOut, newErr)
		for i, f := range files {
			assertSavedFile(t, got.files[fmt.Sprintf("file%d", i)], f)
		}

		// The old CLI still gets the upload, decoded as UTF-8 text as before
		// multipart moved to binary, so its file parts are not byte-exact.
		legacy := oldApp.next(t, 45*time.Second, oldOut, oldErr)
		assert.Equal(t, contentType, legacy.contentType)
		legacyPNG, err := os.ReadFile(legacy.files["file0"])
		require.NoError(t, err)
		assert.NotEqual(t, files[0].data, legacyPNG, "the old CLI cannot receive non-UTF-8 bytes intact")
	})
}

// newSeparateCLIClient returns a runner with its own config file and its own
// `hookdeck ci` login, so it is a separate CLI client. Event ids hash the CLI
// client id, so two listeners sharing one client collapse into a single event
// delivered to only one of them. The new config starts as a copy of base's so
// settings such as api_base carry over.
func newSeparateCLIClient(t *testing.T, base *CLIRunner) *CLIRunner {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "legacy-cli-config.toml")
	if base.configPath != "" {
		if existing, err := os.ReadFile(base.configPath); err == nil {
			require.NoError(t, os.WriteFile(configPath, existing, 0o600))
		}
	}
	return NewCLIRunnerWithConfigPath(t, configPath)
}

// assertEventFailedWithCode waits for an event on the connection to fail with
// the given error code.
func assertEventFailedWithCode(t *testing.T, s binaryListenSetup, code string) {
	t.Helper()
	var last []map[string]any
	for i := 0; i < propagationAttempts; i++ {
		var resp struct {
			Models []map[string]any `json:"models"`
		}
		require.NoError(t, s.cli.RunJSON(&resp, "gateway", "event", "list", "--connection-id", s.connID, "--limit", "20"))
		last = resp.Models
		for _, e := range resp.Models {
			if e["status"] == "FAILED" && e["error_code"] == code {
				return
			}
		}
		time.Sleep(propagationInterval)
	}
	raw, _ := json.Marshal(last)
	t.Fatalf("no event on %s failed with %s; events: %s", s.connID, code, raw)
}
