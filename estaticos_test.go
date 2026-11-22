package main

import (
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
