package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// perfilDePrueba is a trimmed real profile, with what must never reach the
// page: coordinates, postal code, ids, registration date and gender.
const perfilDePrueba = `{"pageProps":{"user":{"id":"ejk4rqmr79zx","microName":"PhoneLab Store V.","type":"normal",
 "webSlug":"chollop-474861123","featured":true,"registerDate":1767869956000,"gender":"M",
 "avatarImage":"https://cdn.wallapop.com/images/13/7u/px/__/c13p474861123/i3621427126.jpg?pictureSize=W320",
 "location":{"city":"Móstoles","zip":"28930","approxRadius":1,"countryCode":"ES","coordinates":{"latitude":40.3281,"longitude":-3.8746}},
 "extraInfoPro":{"description":"iPhone seleccionados y revisados.\nFotos reales."},
 "badgeType":"pro","isVacationModeEnabled":false,"sellerType":"PRO","verified":true},
 "userStats":{"ratings":{"reviews":100},"counters":{"publish":10,"buys":2,"sells":23,"reviews":17,"reportsReceived":1},"ratingAverage":5}}}`

// articulosDePrueba is a page of a seller's items as the API answers.
func articulosDePrueba(n int, siguiente string) string {
	var items []string
	for i := 0; i < n; i++ {
		items = append(items, fmt.Sprintf(`{"id":"i%d","title":"iPhone %d","category_id":"24200","slug":"iphone-%d-1325721908",
 "images":[{"id":"x","average_color":"13C1AC","urls":{"small":"https://cdn.wallapop.com/images/10420/lx/at/__/c1/i%d.jpg?pictureSize=W320"}}],
 "price":{"amount":550.0,"currency":"EUR"},"bump":{"type":"city"},
 "shipping":{"item_is_shippable":true,"user_allows_shipping":true},"is_refurbished":{"flag":false},"has_warranty":%v}`, i, i, i, i, i == 1))
	}
	meta := "{}"
	if siguiente != "" {
		meta = fmt.Sprintf(`{"next":%q}`, siguiente)
	}
	return fmt.Sprintf(`{"data":[%s],"meta":%s}`, strings.Join(items, ","), meta)
}

// vendedorEnWeb serves the test profile, and its items in pages: no cursor
// gives page 1 and cursor "c<n>" page n; each page has 40 items and a next
// cursor until paginas. It records the cursors asked for.
func vendedorEnWeb(t *testing.T, paginas int) (*webDePrueba, *[]string) {
	t.Helper()
	web := &webDePrueba{build: "b1", perfil: perfilDePrueba}
	cursores := &[]string{}
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/users/ejk4rqmr79zx/items" {
			t.Errorf("unexpected API request %s", r.URL)
		}
		since := r.URL.Query().Get("since")
		*cursores = append(*cursores, since)
		n := 1
		if since != "" {
			_, _ = fmt.Sscanf(since, "c%d", &n)
		}
		siguiente := ""
		if n < paginas {
			siguiente = fmt.Sprintf("c%d", n+1)
		}
		_, _ = w.Write([]byte(articulosDePrueba(40, siguiente)))
	}, web.ServeHTTP)
	return web, cursores
}

const rutaVendedor = "/user/chollop-474861123"

func TestASellerPageShowsTheProfileAndItems(t *testing.T) {
	vendedorEnWeb(t, 1)
	w := pedirRuta(t, rutaVendedor)
	html := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	for _, texto := range []string{
		"<title>PhoneLab Store V. · wowpop</title>",
		`<h1 class="perfil-nombre">PhoneLab Store V.</h1>`,
		`chapa-envio">Profesional`, `chapa-garantia">Verificado`,
		`★</span> 5,0 · 17 valoraciones · 23 ventas · 2 compras`,
		"<br>Móstoles",
		"iPhone seleccionados y revisados.\nFotos reales.",
		`src="/foto/images/13/7u/px/__/c13p474861123/i3621427126.jpg?ancho=W320"`,
		`href="https://es.wallapop.com/user/chollop-474861123" rel="noreferrer noopener"`,
		`href="/item/iphone-0-1325721908" target="_blank" rel="noopener"`,
		`src="/foto/images/10420/lx/at/__/c1/i0.jpg?ancho=W320"`,
		"550 €", `chapa-destacado">Destacado`,
	} {
		if !strings.Contains(html, texto) {
			t.Errorf("missing %q", texto)
		}
	}
	if n := strings.Count(html, `<a class="tarjeta`); n != 40 {
		t.Errorf("%d cards, want 40", n)
	}
	if strings.Count(html, `chapa-garantia">Garantía`) != 1 {
		t.Error("only the item with a warranty says so")
	}
	if strings.Contains(html, `class="paginas"`) {
		t.Error("a single page has no page links")
	}
}

func TestASellerPageNeverShowsPrivateData(t *testing.T) {
	vendedorEnWeb(t, 1)
	html := pedirRuta(t, rutaVendedor).Body.String()
	for _, prohibido := range []string{"28930", "40.32", "-3.87", "ejk4rqmr79zx", "1767869956", "cdn.wallapop.com", "Hombre", `"M"`} {
		if strings.Contains(html, prohibido) {
			t.Errorf("the page contains %q", prohibido)
		}
	}
	if strings.Count(html, "wallapop.com") != 1 {
		t.Error("the only mention of wallapop.com is the link to the profile there")
	}
}

func TestASellersItemsHaveNumberedPages(t *testing.T) {
	_, cursores := vendedorEnWeb(t, 3)
	html := pedirRuta(t, rutaVendedor).Body.String()
	clave := claveDe(t, html)
	if !strings.Contains(html, `href="/user/chollop-474861123?b=`+clave+`&amp;p=2" rel="next"`) {
		t.Fatal("no link to page 2")
	}
	for _, n := range []int{2, 3, 1, 3} {
		w := pedirRuta(t, fmt.Sprintf("%s?b=%s&p=%d", rutaVendedor, clave, n))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), fmt.Sprintf(`aria-current="page">%d<`, n)) {
			t.Fatalf("page %d: %d", n, w.Code)
		}
	}
	want := []string{"", "c2", "c3", "", "c3"}
	if fmt.Sprint(*cursores) != fmt.Sprint(want) {
		t.Errorf("cursors %v, want %v", *cursores, want)
	}
}

func TestUnknownSellerPagesSayTheyExpired(t *testing.T) {
	_, cursores := vendedorEnWeb(t, 3)
	w := pedirRuta(t, rutaVendedor+"?b=0123456789abcdef&p=2")
	if w.Code != http.StatusGone || !strings.Contains(w.Body.String(), `href="/user/chollop-474861123"`) || len(*cursores) != 0 {
		t.Fatalf("got %d, %d item requests", w.Code, len(*cursores))
	}
	// A search's key does not open a seller's pages.
	paginasDePrueba(t)
	clave := claveDe(t, pedirRuta(t, "/search?keywords=x").Body.String())
	vendedorEnWeb(t, 3)
	if w := pedirRuta(t, rutaVendedor+"?b="+clave+"&p=2"); w.Code != http.StatusGone {
		t.Errorf("a search key on a seller: %d", w.Code)
	}
}

func TestMissingSellersAre404(t *testing.T) {
	web, cursores := vendedorEnWeb(t, 1)
	web.estadoPagina = http.StatusNotFound
	w := pedirRuta(t, rutaVendedor)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "Perfil no encontrado") || len(*cursores) != 0 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestBadSellerSlugsNeverReachWallapop(t *testing.T) {
	web, cursores := vendedorEnWeb(t, 1)
	for _, ruta := range []string{"/user/", "/user/a.b", "/user/a%2Fb", "/user/" + strings.Repeat("a", 201)} {
		if w := pedirRuta(t, ruta); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", ruta, w.Code)
		}
	}
	if web.paginas+web.pedidos+len(*cursores) != 0 {
		t.Error("requests reached Wallapop")
	}
}

func TestSellerItemErrorsAre502(t *testing.T) {
	web := &webDePrueba{build: "b1", perfil: perfilDePrueba}
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }, web.ServeHTTP)
	if w := pedirRuta(t, rutaVendedor); w.Code != http.StatusBadGateway {
		t.Fatalf("got %d", w.Code)
	}
}

func TestBadCursorsAreNeverSentBack(t *testing.T) {
	web := &webDePrueba{build: "b1", perfil: perfilDePrueba}
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(articulosDePrueba(2, "a&b=c")))
	}, web.ServeHTTP)
	if strings.Contains(pedirRuta(t, rutaVendedor).Body.String(), `rel="next"`) {
		t.Error("an invalid cursor must not become a next page")
	}
}

func TestAnEmptyShopSaysSo(t *testing.T) {
	web := &webDePrueba{build: "b1", perfil: perfilDePrueba}
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"meta":{}}`))
	}, web.ServeHTTP)
	if html := pedirRuta(t, rutaVendedor).Body.String(); !strings.Contains(html, "No tiene nada en venta") {
		t.Error("an empty shop must say so")
	}
}

func TestRatingsAreStarsOutOfFive(t *testing.T) {
	for puntos, want := range map[float64]string{100: "5,0", 96: "4,8", 50: "2,5", 1: "0,1", 0: "", -3: "", 101: ""} {
		if got := formatearNota(puntos); got != want {
			t.Errorf("%v: %q, want %q", puntos, got, want)
		}
	}
	if inicial("ñandú") != "Ñ" || inicial("") != "" {
		t.Error("initials")
	}
}
