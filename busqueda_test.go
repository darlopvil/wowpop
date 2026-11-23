package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// respuestaDePrueba is a trimmed real answer: three items covering every
// badge, one without photos, and the seller's coordinates, which must
// never reach the page.
const respuestaDePrueba = `{"data":{"section":{"type":"organic_search_results","items":[
{"id":"e65qx5p87p6o","title":"Nintendo Game Boy Pocket Plateada","price":{"amount":30.0,"currency":"EUR"},
 "images":[{"id":"n","average_color":"13C1AC","urls":{"small":"https://cdn.wallapop.com/images/10420/lx/zr/__/c10420p1326885872/i6880198806.jpg?pictureSize=W320"}}],
 "reserved":{"flag":true},"location":{"latitude":43.49524666488559,"longitude":-8.199226283755593,"postal_code":"15570","city":"Ferrol"},
 "shipping":{"item_is_shippable":true,"user_allows_shipping":true},"bump":{"type":"none"},"has_warranty":{"flag":false},
 "web_slug":"nintendo-game-boy-pocket-plateada-1326885872"},
{"id":"b","title":"Game Boy Color <b>con juegos</b>","price":{"amount":1250.0,"currency":"EUR"},
 "images":[{"urls":{"small":"https://cdn.wallapop.com/images/10420/ab/cd/__/c1/i2.jpg?pictureSize=W320"}}],
 "reserved":{"flag":false},"location":{"city":"Sevilla"},"shipping":{"item_is_shippable":true,"user_allows_shipping":false},
 "bump":{"type":"city"},"has_warranty":{"flag":true},"web_slug":"game-boy-color-2"},
{"id":"c","title":"Cartucho","price":{"amount":12.99,"currency":"EUR"},"images":[],"reserved":{"flag":false},
 "location":{"city":""},"shipping":{},"bump":{},"has_warranty":{"flag":false},"web_slug":"cartucho-3"}
]}},"meta":{"next_page":"eyJhbGciOiJIUzI1NiJ9.eyJzIjoiZyJ9.c2lnbmF0dXJl"}}`

// buscar runs /search against a fake API and returns the page and the query
// the API received.
func buscar(t *testing.T, ruta string, estado int, cuerpo string) (*httptest.ResponseRecorder, url.Values) {
	t.Helper()
	var recibida url.Values
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		recibida = r.URL.Query()
		w.WriteHeader(estado)
		_, _ = w.Write([]byte(cuerpo))
	}, nil)
	return pedirRuta(t, ruta), recibida
}

func TestSearchShowsACardPerItem(t *testing.T) {
	w, _ := buscar(t, "/search?keywords=game+boy", http.StatusOK, respuestaDePrueba)
	html := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	for _, texto := range []string{
		`«game boy»`,
		`value="game boy"`,
		`href="/item/nintendo-game-boy-pocket-plateada-1326885872" target="_blank" rel="noopener"`,
		`src="/foto/images/10420/lx/zr/__/c10420p1326885872/i6880198806.jpg?ancho=W320"`,
		"Nintendo Game Boy Pocket Plateada",
		"30\u00a0€", "1.250\u00a0€", "12,99\u00a0€",
		"Ferrol", "Sevilla",
		`class="tarjeta tarjeta-reservada"`,
		`chapa-reservado">Reservado`, `chapa-envio">Envío`, `chapa-garantia">Garantía`, `chapa-destacado">Destacado`,
		"Game Boy Color &lt;b&gt;con juegos&lt;/b&gt;",
	} {
		if !strings.Contains(html, texto) {
			t.Errorf("missing %q", texto)
		}
	}
	if n := strings.Count(html, `<a class="tarjeta`); n != 3 {
		t.Errorf("%d cards, want 3", n)
	}
	if strings.Count(html, "chapa-envio") != 1 {
		t.Error("shipping is only offered when the seller allows it")
	}
}

func TestSearchPagesNeverPointToWallapopOrShowCoordinates(t *testing.T) {
	w, _ := buscar(t, "/search?keywords=game+boy", http.StatusOK, respuestaDePrueba)
	html := w.Body.String()
	for _, prohibido := range []string{"wallapop.com", "43.49", "-8.19", "15570", "13C1AC"} {
		if strings.Contains(html, prohibido) {
			t.Errorf("the page contains %q", prohibido)
		}
	}
}

func TestSearchSendsTheChosenOrder(t *testing.T) {
	for clave, api := range map[string]string{"": "most_relevance", "nuevos": "newest", "baratos": "price_low_to_high", "caros": "price_high_to_low", "inventado": "most_relevance"} {
		w, q := buscar(t, "/search?keywords=gameboy&orden="+clave, http.StatusOK, respuestaDePrueba)
		if q.Get("order_by") != api || q.Get("keywords") != "gameboy" {
			t.Errorf("orden=%q sent order_by=%q", clave, q.Get("order_by"))
		}
		if strings.Count(w.Body.String(), `aria-current="true"`) != 1 {
			t.Errorf("orden=%q: exactly one order should be marked", clave)
		}
	}
}

func TestSearchTidiesTheKeywords(t *testing.T) {
	_, q := buscar(t, "/search?keywords=++game%09+boy++", http.StatusOK, respuestaDePrueba)
	if q.Get("keywords") != "game boy" {
		t.Errorf("sent %q", q.Get("keywords"))
	}
}

func TestEmptySearchesGoBackHome(t *testing.T) {
	for _, ruta := range []string{"/search", "/search?keywords=", "/search?keywords=+++"} {
		w := pedirRuta(t, ruta)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
			t.Errorf("%s: %d to %q", ruta, w.Code, w.Header().Get("Location"))
		}
	}
}

func TestOverlongSearchesAreRefused(t *testing.T) {
	llamadas := 0
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) { llamadas++ }, nil)
	w := pedirRuta(t, "/search?keywords="+strings.Repeat("a", limitePalabras+1))
	if w.Code != http.StatusBadRequest || llamadas != 0 {
		t.Fatalf("%d, %d calls", w.Code, llamadas)
	}
}

func TestNoResultsSaysSo(t *testing.T) {
	w, _ := buscar(t, "/search?keywords=zzzz", http.StatusOK, `{"data":{"section":{"items":[]}},"meta":{"next_page":"a.b.c"}}`)
	html := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(html, "Sin resultados") || strings.Contains(html, "Siguiente") {
		t.Fatalf("got %d", w.Code)
	}
}

func TestWallapopErrorsAre502(t *testing.T) {
	for _, c := range []struct {
		estado int
		cuerpo string
	}{{http.StatusInternalServerError, "{}"}, {http.StatusForbidden, "<html>"}, {http.StatusOK, "not json"}} {
		w, _ := buscar(t, "/search?keywords=x", c.estado, c.cuerpo)
		if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "Wallapop no responde") {
			t.Errorf("Wallapop %d: got %d", c.estado, w.Code)
		}
	}
}

func TestPricesAreWrittenTheSpanishWay(t *testing.T) {
	for cantidad, want := range map[float64]string{
		0: "0\u00a0€", 5: "5\u00a0€", 12.99: "12,99\u00a0€", 12.5: "12,50\u00a0€", 999: "999\u00a0€",
		1250: "1.250\u00a0€", 1234567.8: "1.234.567,80\u00a0€", 0.1 + 0.2: "0,30\u00a0€", -1: "",
	} {
		if got := formatearPrecio(cantidad, "EUR"); got != want {
			t.Errorf("%v: %q, want %q", cantidad, got, want)
		}
	}
	if got := formatearPrecio(10, "GBP"); got != "10\u00a0GBP" {
		t.Errorf("other currencies keep their code: %q", got)
	}
}

// paginasDePrueba is a fake API with endless pages: page n is one item,
// "Pagina n", and its next_page token is "a.b.<n+1>". It records the
// queries it receives.
func paginasDePrueba(t *testing.T) *[]url.Values {
	t.Helper()
	recibidas := &[]url.Values{}
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		*recibidas = append(*recibidas, q)
		n := 1
		if tok := q.Get("next_page"); tok != "" {
			_, _ = fmt.Sscanf(tok, "a.b.%d", &n)
		}
		fmt.Fprintf(w, `{"data":{"section":{"items":[{"title":"Pagina %d","web_slug":"s"}]}},"meta":{"next_page":"a.b.%d"}}`, n, n+1)
	}, nil)
	return recibidas
}

var reClaveEnPagina = regexp.MustCompile(`b=([0-9a-f]{16})`)

func claveDe(t *testing.T, html string) string {
	t.Helper()
	m := reClaveEnPagina.FindStringSubmatch(html)
	if m == nil {
		t.Fatal("no page links")
	}
	return m[1]
}

func TestFirstPageLinksTheSecond(t *testing.T) {
	paginasDePrueba(t)
	html := pedirRuta(t, "/search?keywords=game+boy&orden=baratos").Body.String()
	clave := claveDe(t, html)
	for _, texto := range []string{
		`href="/search?b=` + clave + `&amp;keywords=game&#43;boy&amp;orden=baratos&amp;p=2" rel="next"`,
		`aria-current="page">1<`,
	} {
		if !strings.Contains(html, texto) {
			t.Errorf("missing %s", texto)
		}
	}
	if strings.Contains(html, `rel="prev"`) {
		t.Error("the first page has no previous one")
	}
}

func TestPagesCanBeRevisitedWithoutLosingTheOthers(t *testing.T) {
	recibidas := paginasDePrueba(t)
	clave := claveDe(t, pedirRuta(t, "/search?keywords=x").Body.String())
	pagina := func(n int) string {
		t.Helper()
		w := pedirRuta(t, fmt.Sprintf("/search?keywords=x&b=%s&p=%d", clave, n))
		if w.Code != http.StatusOK {
			t.Fatalf("page %d: %d", n, w.Code)
		}
		return w.Body.String()
	}
	pagina(2)
	html := pagina(3)
	if !strings.Contains(html, "Pagina 3") || !strings.Contains(html, `p=4" rel="next"`) || !strings.Contains(html, `p=2" rel="prev"`) {
		t.Fatal("page 3 is wrong")
	}
	html = pagina(1)
	if !strings.Contains(html, "Pagina 1") || !strings.Contains(html, `p=4">4<`) {
		t.Error("back on page 1, the later pages must still be linked")
	}
	html = pagina(2)
	if !strings.Contains(html, "Pagina 2") || !strings.Contains(html, `aria-current="page">2<`) {
		t.Error("page 2 revisited")
	}

	// The token of each page is sent alone; page 1 again is the same search.
	q := *recibidas
	if q[1].Get("next_page") != "a.b.2" || len(q[1]) != 1 || q[2].Get("next_page") != "a.b.3" {
		t.Errorf("tokens sent: %v %v", q[1], q[2])
	}
	if q[3].Get("search_id") != q[0].Get("search_id") || q[3].Get("next_page") != "" {
		t.Errorf("page 1 again should repeat search_id %q, sent %v", q[0].Get("search_id"), q[3])
	}
}

func TestDistantPagesAreShortened(t *testing.T) {
	paginasDePrueba(t)
	clave := claveDe(t, pedirRuta(t, "/search?keywords=x").Body.String())
	for n := 2; n <= 9; n++ {
		pedirRuta(t, fmt.Sprintf("/search?keywords=x&b=%s&p=%d", clave, n))
	}
	html := pedirRuta(t, fmt.Sprintf("/search?keywords=x&b=%s&p=5", clave)).Body.String()
	for _, n := range []int{1, 3, 4, 5, 6, 7, 10} {
		if !strings.Contains(html, fmt.Sprintf(`>%d<`, n)) {
			t.Errorf("page %d is not linked", n)
		}
	}
	for _, n := range []int{2, 8, 9} {
		if strings.Contains(html, fmt.Sprintf(`>%d<`, n)) {
			t.Errorf("page %d should be left out", n)
		}
	}
	if strings.Count(html, "paginas-hueco") != 2 {
		t.Error("each gap gets one ellipsis")
	}
}

func TestUnknownSearchesStartAgain(t *testing.T) {
	recibidas := paginasDePrueba(t)
	for _, ruta := range []string{
		"/search?keywords=x&b=0123456789abcdef&p=3",
		"/search?keywords=x&b=nope&p=2",
		"/search?keywords=x&p=7",
	} {
		antes := len(*recibidas)
		w := pedirRuta(t, ruta)
		switch {
		case strings.Contains(ruta, "b=0123456789abcdef") || strings.Contains(ruta, "b=nope"):
			if w.Code != http.StatusGone || len(*recibidas) != antes || !strings.Contains(w.Body.String(), "Volver a buscar") {
				t.Errorf("%s: %d, %d calls", ruta, w.Code, len(*recibidas)-antes)
			}
		default:
			if w.Code != http.StatusOK || (*recibidas)[antes].Get("keywords") != "x" {
				t.Errorf("%s should be a new search", ruta)
			}
		}
	}
}

func TestAKeyOnlyOpensItsOwnSearch(t *testing.T) {
	recibidas := paginasDePrueba(t)
	clave := claveDe(t, pedirRuta(t, "/search?keywords=x").Body.String())
	pedirRuta(t, "/search?keywords=x&b="+clave+"&p=2")
	for _, ruta := range []string{"/search?keywords=otra&b=" + clave + "&p=2", "/search?keywords=x&orden=caros&b=" + clave + "&p=2"} {
		antes := len(*recibidas)
		if w := pedirRuta(t, ruta); w.Code != http.StatusGone || len(*recibidas) != antes {
			t.Errorf("%s: %d", ruta, w.Code)
		}
	}
	if w := pedirRuta(t, "/search?keywords=x&b="+clave+"&p=9"); w.Code != http.StatusGone {
		t.Errorf("a page not reached yet: %d", w.Code)
	}
}

func TestBadTokensFromWallapopAreNeverSentBack(t *testing.T) {
	w, _ := buscar(t, "/search?keywords=x", http.StatusOK, `{"data":{"section":{"items":[{"title":"a"}]}},"meta":{"next_page":"../../x?y"}}`)
	if strings.Contains(w.Body.String(), "Siguiente") {
		t.Error("an invalid token must not become a next page")
	}
}

func TestExpiredTokensExplainWhy(t *testing.T) {
	paso := 0
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		paso++
		if paso > 1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"section":{"items":[{"title":"a"}]}},"meta":{"next_page":"a.b.c"}}`))
	}, nil)
	clave := claveDe(t, pedirRuta(t, "/search?keywords=x").Body.String())
	w := pedirRuta(t, "/search?keywords=x&b="+clave+"&p=2")
	if w.Code != http.StatusGone || !strings.Contains(w.Body.String(), "caducado") {
		t.Fatalf("got %d", w.Code)
	}
}
