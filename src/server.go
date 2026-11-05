// A private web server for the app window. It only listens on this computer, and
// every request must carry a secret that's created fresh each time the app starts,
// so websites you visit can't read or change your crosshairs.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/draw"
	"image/png"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed ui
var uiFiles embed.FS

const (
	cookieName     = "freezyaim_session"
	maxBody        = 1 << 20
	maxPreviewSide = 1600 // Stops a very large crosshair making a huge preview.
	contentPolicy  = "default-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"
)

type Server struct {
	store *Store
	addr  string
	token string
}

func StartServer(store *Store) (*Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	s := &Server{store: store, addr: listener.Addr().String(), token: hex.EncodeToString(secret)}
	server := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go server.Serve(listener)
	return s, nil
}

// URL opens the app and signs the window in.
func (s *Server) URL() string { return "http://" + s.addr + "/?t=" + s.token }

func (s *Server) sameSecret(value string) bool {
	return subtle.ConstantTimeCompare([]byte(value), []byte(s.token)) == 1
}

// guard rejects anything that isn't the app window.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != s.addr {
			http.Error(w, "Not allowed.", http.StatusForbidden)
			return
		}
		if r.URL.Path == "/" && s.sameSecret(r.URL.Query().Get("t")) {
			http.SetCookie(w, &http.Cookie{
				Name: cookieName, Value: s.token, Path: "/",
				HttpOnly: true, SameSite: http.SameSiteStrictMode,
			})
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		cookie, err := r.Cookie(cookieName)
		if err != nil || !s.sameSecret(cookie.Value) {
			http.Error(w, "Open this window from the Freezy Aim tray icon.", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodPost {
			origin := r.Header.Get("Origin")
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") ||
				(origin != "" && origin != "http://"+s.addr) {
				http.Error(w, "Not allowed.", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("Content-Security-Policy", contentPolicy)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// actionRequest holds the fields any button in the app window can send.
type actionRequest struct {
	OverlayUpdate
	OptionsUpdate
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	On     bool            `json:"on"`
	Config json.RawMessage `json:"config"`
	Main   string          `json:"main"`
	Accent string          `json:"accent"`
	Action string          `json:"action"`
	Bar    int             `json:"bar"`
}

// action runs a change, then replies with the fresh state and any new ID.
func (s *Server) action(run func(actionRequest) (string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request actionRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
		if err := decoder.Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "That request couldn't be read."})
			return
		}
		id, err := run(request)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "state": s.store.State()})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func (s *Server) Handler() http.Handler {
	assets, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		panic(err) // The files are built in, so this can't happen at run time.
	}
	files := http.FileServerFS(assets)
	nothing := func(do func(actionRequest) error) http.HandlerFunc {
		return s.action(func(request actionRequest) (string, error) { return "", do(request) })
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.page(assets))
	for _, path := range []string{"/app.js", "/motion.js", "/editor.js", "/draw.js", "/settings.js", "/app.css", "/icon.png", "/rings.svg", "/fonts/"} {
		mux.Handle("GET "+path, files)
	}
	mux.HandleFunc("GET /logo.png", s.logo)
	mux.HandleFunc("GET /render.png", s.render)
	mux.HandleFunc("GET /api/measure", s.measure)
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.store.State())
	})
	catalog := NewCatalog(s.store.overlay.SystemFeatures())
	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, catalog)
	})
	mux.HandleFunc("POST /api/options", nothing(func(r actionRequest) error { return s.store.SetOptions(r.OptionsUpdate) }))
	mux.HandleFunc("POST /api/quit", nothing(func(actionRequest) error {
		s.store.overlay.Quit()
		return nil
	}))
	mux.HandleFunc("POST /api/restore", nothing(func(actionRequest) error { return s.store.RestoreDefaults() }))
	mux.HandleFunc("POST /api/use", nothing(func(r actionRequest) error { return s.store.Use(r.ID) }))
	mux.HandleFunc("POST /api/favorite", nothing(func(r actionRequest) error { return s.store.SetFavorite(r.ID, r.On) }))
	mux.HandleFunc("POST /api/delete", nothing(func(r actionRequest) error { return s.store.Delete(r.ID) }))
	mux.HandleFunc("POST /api/theme", nothing(func(r actionRequest) error {
		return s.store.SetTheme(Theme{Main: r.Main, Accent: r.Accent})
	}))
	mux.HandleFunc("POST /api/overlay", nothing(func(r actionRequest) error { return s.store.SetOverlay(r.OverlayUpdate) }))
	mux.HandleFunc("POST /api/open-folder", nothing(func(actionRequest) error {
		s.store.overlay.OpenFolder()
		return nil
	}))
	mux.HandleFunc("POST /api/window", nothing(func(r actionRequest) error {
		s.store.overlay.Window(r.Action, r.Bar)
		return nil
	}))
	mux.HandleFunc("POST /api/draft", nothing(func(r actionRequest) error {
		if len(r.Config) == 0 {
			return s.store.SetDraft(nil)
		}
		shape, err := ParseConfig(r.Config)
		if err != nil {
			return err
		}
		return s.store.SetDraft(&Crosshair{Name: cleanName(r.Name), Config: shape})
	}))
	mux.HandleFunc("POST /api/copy", s.action(func(r actionRequest) (string, error) { return s.store.Copy(r.ID) }))
	mux.HandleFunc("POST /api/crosshair", s.action(func(r actionRequest) (string, error) {
		shape := DefaultConfig()
		if len(r.Config) > 0 {
			var err error
			if shape, err = ParseConfig(r.Config); err != nil {
				return "", err
			}
		}
		return s.store.Save(r.ID, r.Name, shape)
	}))
	return s.guard(mux)
}

// page sends the app with its colours built in, so it never flashes the wrong ones.
func (s *Server) page(assets fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		html, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		state := s.store.State()
		html = bytes.Replace(html, []byte("/*theme*/"), []byte(state.ThemeCSS), 1)
		html = bytes.Replace(html, []byte("/*look*/"), []byte(state.Options.Background), 1)
		html = bytes.Replace(html, []byte("/*title*/"), []byte(state.TitleColor), 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(html)
	}
}

// logo sends the logo.png from the data folder. Without one, the app shows its own.
func (s *Server) logo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, s.store.logoPath)
}

// render draws a crosshair with the same code as the overlay, so what you see in the
// app is what appears on screen. `scale` redraws it larger, so previews stay sharp.
func (s *Server) render(w http.ResponseWriter, r *http.Request) {
	config, err := ParseConfig([]byte(r.URL.Query().Get("c")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	scale, _ := strconv.Atoi(r.URL.Query().Get("scale"))
	w.Header().Set("Cache-Control", "max-age=3600")
	w.Header().Set("Content-Type", "image/png")
	picture := Render(config.Scaled(previewScale(config, scale)))
	if config.Opacity < 1 {
		// On screen, the overlay window fades the crosshair. Previews have to do it themselves.
		for index := 3; index < len(picture.Pix); index += 4 {
			picture.Pix[index] = byte(float64(picture.Pix[index]) * config.Opacity)
		}
	}
	writePNG(w, picture)
}

// previewScale keeps a preview within its size limit.
func previewScale(c Config, scale int) int {
	return min(max(scale, 1), max(maxPreviewSide/windowSide(c), 1))
}

// measure reports the real size of a preview, for lining it up with the drawing area.
func (s *Server) measure(w http.ResponseWriter, r *http.Request) {
	config, err := ParseConfig([]byte(r.URL.Query().Get("c")))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	wanted, _ := strconv.Atoi(r.URL.Query().Get("scale"))
	scale := previewScale(config, wanted)
	writeJSON(w, http.StatusOK, map[string]int{"side": windowSide(config.Scaled(scale)), "scale": scale})
}

// iconPNG returns the reticle used for the tray icon.
func iconPNG() ([]byte, error) { return uiFiles.ReadFile("ui/icon.png") }

// appIcon returns the tray icon as pixels for Windows.
func appIcon() (Image, error) {
	data, err := iconPNG()
	if err != nil {
		return Image{}, err
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return Image{}, err
	}
	picture := image.NewNRGBA(decoded.Bounds())
	draw.Draw(picture, picture.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	img := Image{Side: picture.Bounds().Dx(), Pix: bytes.Clone(picture.Pix)}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+2] = img.Pix[i+2], img.Pix[i]
	}
	return img, nil
}

// pngBuffers keeps the encoder's large working memory between pictures.
type pngBuffers struct{ pool sync.Pool }

func (b *pngBuffers) Get() *png.EncoderBuffer {
	buffer, _ := b.pool.Get().(*png.EncoderBuffer)
	return buffer
}

func (b *pngBuffers) Put(buffer *png.EncoderBuffer) { b.pool.Put(buffer) }

// pngEncoder favours speed because previews update as you drag.
var pngEncoder = png.Encoder{CompressionLevel: png.BestSpeed, BufferPool: &pngBuffers{}}

func writePNG(w io.Writer, img Image) error {
	out := image.NewNRGBA(image.Rect(0, 0, img.Side, img.Side))
	for i := 0; i < len(out.Pix); i += 4 {
		out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = img.Pix[i+2], img.Pix[i+1], img.Pix[i], img.Pix[i+3]
	}
	return pngEncoder.Encode(w, out)
}
