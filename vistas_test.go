package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pedirRuta(t *testing.T, ruta string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	rutas().ServeHTTP(w, httptest.NewRequest(http.MethodGet, ruta, nil))
	return w
}

func TestHomePageHasTheBigSearchBox(t *testing.T) {
	w := pedirRuta(t, "/")
	html := w.Body.String()
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("home: %d, %q", w.Code, w.Header().Get("Content-Type"))
	}
	if strings.Count(html, `name="keywords"`) != 1 || !strings.Contains(html, `action="/search"`) {
		t.Error("the home page should have exactly one search box, its own, pointing to /search")
	}
}

func TestOtherPagesHaveTheHeaderSearchBox(t *testing.T) {
	html := pedirRuta(t, "/about").Body.String()
	if !strings.Contains(html, `<form class="buscador" action="/search"`) || !strings.Contains(html, `name="keywords"`) {
		t.Error("the header search box is missing")
	}
}

func TestEveryPageHasTheFooterAndTheAboutLink(t *testing.T) {
	for _, ruta := range []string{"/", "/about", "/no-existe"} {
		html := pedirRuta(t, ruta).Body.String()
		for _, texto := range []string{
			`href="/about"`,
			`href="/about#aviso-legal"`,
			`https://github.com/darlopvil/wowpop`,
			"Proyecto independiente, sin relación con Wallapop.",
		} {
			if !strings.Contains(html, texto) {
				t.Errorf("%s: missing %q", ruta, texto)
			}
		}
	}
}

func TestAboutHasTheLegalNoticeAndLicences(t *testing.T) {
	html := pedirRuta(t, "/about").Body.String()
	for _, texto := range []string{
		`id="aviso-legal"`,
		"sin relación con Wallapop",
		"aquí no se compra, no se vende ni se contacta con nadie",
		"no guarda datos de los vendedores",
		"Tu navegador solo se conecta a wowpop",
		"Ligera:",
		"Frutiger Aero",
		"GNU AGPL-3.0",
		estatico("fuentes/OFL-OpenSans.txt"),
	} {
		if !strings.Contains(html, texto) {
			t.Errorf("/about: missing %q", texto)
		}
	}
}

func TestUnknownPagesAre404(t *testing.T) {
	w := pedirRuta(t, "/no-existe")
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "Página no encontrada") {
		t.Fatalf("got %d", w.Code)
	}
}

func TestPagesHaveNoScripts(t *testing.T) {
	for _, ruta := range []string{"/", "/about", "/no-existe"} {
		if strings.Contains(pedirRuta(t, ruta).Body.String(), "<script") {
			t.Errorf("%s has a script", ruta)
		}
	}
}

func TestFaviconIcoPointsToTheSVG(t *testing.T) {
	w := pedirRuta(t, "/favicon.ico")
	if w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != estatico("favicon.svg") {
		t.Fatalf("got %d to %q", w.Code, w.Header().Get("Location"))
	}
}
