package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pedirConCabecera(t *testing.T, ruta, cabecera, valor string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, ruta, nil)
	r.Header.Set(cabecera, valor)
	rutas().ServeHTTP(w, r)
	return w
}

func TestPagesLinkVersionedStaticFiles(t *testing.T) {
	html := pedirRuta(t, "/").Body.String()
	for _, nombre := range []string{"estilo.css", "favicon.svg"} {
		if v := estaticos["/static/"+nombre].version; v == "" || !strings.Contains(html, `href="/static/`+nombre+`?v=`+v+`"`) {
			t.Errorf("%s not linked with its version", nombre)
		}
	}
}

func TestVersionedStaticFilesAreKeptForAYear(t *testing.T) {
	for _, ruta := range []string{"/static/estilo.css", "/static/fuentes/open-sans.woff2", "/static/favicon.svg"} {
		w := pedirRuta(t, estatico(strings.TrimPrefix(ruta, "/static/")))
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
			t.Errorf("%s: %d, Cache-Control %q", ruta, w.Code, w.Header().Get("Cache-Control"))
		}
	}
	if tipo := pedirRuta(t, estatico("fuentes/open-sans.woff2")).Header().Get("Content-Type"); tipo != "font/woff2" {
		t.Errorf("font served as %q", tipo)
	}
}

func TestTheStylesheetLoadsVersionedFonts(t *testing.T) {
	css := pedirRuta(t, estatico("estilo.css")).Body.String()
	for _, fuente := range []string{"fuentes/open-sans.woff2"} {
		if !strings.Contains(css, `url("`+estatico(fuente)+`")`) {
			t.Errorf("stylesheet does not load %s with its version", fuente)
		}
	}
}

func TestStaticFilesWithoutTheirVersionMustBeChecked(t *testing.T) {
	for _, ruta := range []string{"/static/estilo.css", "/static/estilo.css?v=viejo", "/static/fuentes/OFL-OpenSans.txt"} {
		w := pedirRuta(t, ruta)
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-cache" || w.Header().Get("ETag") == "" {
			t.Fatalf("%s: %d, Cache-Control %q, ETag %q", ruta, w.Code, w.Header().Get("Cache-Control"), w.Header().Get("ETag"))
		}
	}
	etag := pedirRuta(t, "/static/estilo.css").Header().Get("ETag")
	if w := pedirConCabecera(t, "/static/estilo.css", "If-None-Match", etag); w.Code != http.StatusNotModified || w.Body.Len() != 0 {
		t.Fatalf("an unchanged file should be a 304 without body, got %d with %d B", w.Code, w.Body.Len())
	}
}

func TestStaticFoldersAreNotListed(t *testing.T) {
	for _, ruta := range []string{"/static/", "/static/fuentes/", "/static/nada.css"} {
		if w := pedirRuta(t, ruta); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", ruta, w.Code)
		}
	}
}

func TestTextFilesAreGzippedForBrowsersThatAcceptIt(t *testing.T) {
	for _, nombre := range []string{"estilo.css", "arte/mariposa-1.svg", "fuentes/OFL-OpenSans.txt"} {
		ruta := estatico(nombre)
		w := pedirConCabecera(t, ruta, "Accept-Encoding", "br, gzip, deflate")
		if w.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(w.Header().Get("Vary"), "Accept-Encoding") {
			t.Fatalf("%s: Content-Encoding %q, Vary %q", nombre, w.Header().Get("Content-Encoding"), w.Header().Get("Vary"))
		}
		lector, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatal(err)
		}
		descomprimido, _ := io.ReadAll(lector)
		if !bytes.Equal(descomprimido, estaticos[strings.SplitN(ruta, "?", 2)[0]].contenido) {
			t.Errorf("%s: the gzipped body is not the file", nombre)
		}
		if strings.Contains(w.Header().Get("Content-Type"), "gzip") {
			t.Errorf("%s: served as %q", nombre, w.Header().Get("Content-Type"))
		}
	}
}

func TestBrowsersWithoutGzipGetThePlainFile(t *testing.T) {
	for _, cabecera := range []string{"", "identity", "gzip;q=0", "br"} {
		w := pedirConCabecera(t, estatico("estilo.css"), "Accept-Encoding", cabecera)
		if w.Header().Get("Content-Encoding") != "" || !strings.Contains(w.Body.String(), "@font-face") {
			t.Errorf("Accept-Encoding %q: got Content-Encoding %q", cabecera, w.Header().Get("Content-Encoding"))
		}
	}
}

func TestGzippedAndPlainCopiesHaveDifferentETags(t *testing.T) {
	plano := pedirConCabecera(t, estatico("estilo.css"), "Accept-Encoding", "").Header().Get("ETag")
	gz := pedirConCabecera(t, estatico("estilo.css"), "Accept-Encoding", "gzip").Header().Get("ETag")
	if plano == gz {
		t.Fatalf("both copies share the ETag %s", plano)
	}
}

func TestFontsAreNotGzippedAgain(t *testing.T) {
	w := pedirConCabecera(t, estatico("fuentes/open-sans.woff2"), "Accept-Encoding", "gzip")
	if w.Header().Get("Content-Encoding") != "" || w.Header().Get("Vary") != "" {
		t.Fatalf("woff2 is already compressed: Content-Encoding %q, Vary %q", w.Header().Get("Content-Encoding"), w.Header().Get("Vary"))
	}
}

func TestSVGAndTextHaveTheirTypes(t *testing.T) {
	for nombre, tipo := range map[string]string{"arte/info.svg": "image/svg+xml", "fuentes/OFL-OpenSans.txt": "text/plain; charset=utf-8", "estilo.css": "text/css; charset=utf-8"} {
		if got := pedirRuta(t, estatico(nombre)).Header().Get("Content-Type"); got != tipo {
			t.Errorf("%s: %q", nombre, got)
		}
	}
}
