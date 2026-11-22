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
		"Ligera:", "unos 85 KB",
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

func TestHomePageHasTheNatureScene(t *testing.T) {
	html := pedirRuta(t, "/").Body.String()
	for _, texto := range []string{`<body class="inicio">`, `class="escena" aria-hidden="true"`, estatico("arte/mariposa-1.svg"), estatico("arte/pez-dorado.svg"), estatico("arte/pez-angel.svg"), `class="portada-reflejo" aria-hidden="true"`} {
		if !strings.Contains(html, texto) {
			t.Errorf("home: missing %q", texto)
		}
	}
	if html := pedirRuta(t, "/about").Body.String(); strings.Contains(html, `class="inicio"`) || strings.Contains(html, `class="viajero"`) {
		t.Error("only the home page gets the scene")
	}
}

func TestAboutCreditsTheArtwork(t *testing.T) {
	html := pedirRuta(t, "/about").Body.String()
	for _, id := range []string{"346015", "181437", "122389", "237408", "34723", "7982", "351206", "2213", "3320"} {
		if !strings.Contains(html, "https://openclipart.org/detail/"+id) {
			t.Errorf("/about does not credit Openclipart %s", id)
		}
	}
}

func TestEveryPageHasRisingBubbles(t *testing.T) {
	for _, ruta := range []string{"/", "/about", "/no-existe"} {
		if n := strings.Count(pedirRuta(t, ruta).Body.String(), `class="burbuja"`); n != 8 {
			t.Errorf("%s: %d bubbles, the stylesheet places 8", ruta, n)
		}
	}
}

func TestEveryArtworkCreditShowsItsDrawing(t *testing.T) {
	html := pedirRuta(t, "/about").Body.String()
	css := string(estaticos["/static/estilo.css"].contenido)
	for _, clase := range []string{"mariposas", "pez-dorado", "pez-angel", "brotes", "check", "info", "pergamino", "tierra-reciclaje", "tierra"} {
		if !strings.Contains(html, `<li class="dibujo-`+clase+`">`) || !strings.Contains(css, ".dibujo-"+clase+" {") {
			t.Errorf("credit %q has no drawing for its bullet", clase)
		}
	}
}
