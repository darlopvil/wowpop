package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// fichaDePrueba is a trimmed real listing, with the seller's coordinates,
// postal code and ids, which must never reach the page.
const fichaDePrueba = `{"pageProps":{"item":{
 "id":"9jdworlgn96k","title":{"original":"iPhone 13 Blanco | Nuevo","translated":null},
 "description":{"original":"iPhone 13 Blanco | 128GB\n\n<b>Batería</b> al 100%","translated":null},
 "characteristics":"En su caja · Apple · iPhone 13 · 128 GB · Blanco",
 "taxonomies":[{"id":"24200","name":"Tecnología y electrónica"},{"id":"9447","name":"Smartphones"}],
 "userId":"ejk4rqmr79zx","slug":"iphone-13-blanco-nuevo-1326730384","modifiedDate":1795274769000,
 "images":[
  {"id":"a","averageColor":"13C1AC","urls":{"small":"https://cdn.wallapop.com/images/10420/lx/wf/__/c1/i1.jpg?pictureSize=W320","big":"https://cdn.wallapop.com/images/10420/lx/wf/__/c1/i1.jpg?pictureSize=W800"}},
  {"id":"b","urls":{"small":"https://cdn.wallapop.com/images/10420/lx/wf/__/c1/i2.jpg?pictureSize=W320","big":"https://cdn.wallapop.com/images/10420/lx/wf/__/c1/i2.jpg?pictureSize=W800"}},
  {"id":"c","urls":{"small":"https://evil.example/x.jpg","big":"https://evil.example/x.jpg"}}],
 "price":{"cash":{"amount":290,"currency":"EUR"},"financed":null},
 "location":{"latitude":40.328174058896,"longitude":-3.874626104631602,"approximated":true,"countryCode":"ES","city":"Móstoles","postalCode":"28930"},
 "flags":{"reserved":false,"bumped":true,"sold":false,"expired":false},
 "shipping":{"isItemShippable":true,"isShippingAllowedByUser":true},
 "views":161,"favorites":17,
 "condition":{"iconText":"En su caja","text":"En su caja","title":"Estado","value":"in_box"},
 "characteristicsDetails":[{"attribute":"Modelo","value":"iPhone 13"},{"attribute":"Capacidad de almacenamiento","value":"128 GB"}],
 "isRefurbished":false},
 "itemSeller":{"id":"ejk4rqmr79zx","microName":"PhoneLab Store V.","webSlug":"chollop-474861123","registerDate":1767869956000,
  "avatarImage":"https://cdn.wallapop.com/images/13/7u/px/__/c13p474861123/i3621427126.jpg?pictureSize=W320",
  "location":{"city":"Móstoles","zip":"28930","approxRadius":1,"coordinates":{"latitude":40.3281,"longitude":-3.8746}},
  "sellerType":"PRO","gender":"M","stats":{"ratings":{"reviews":100},"counters":{"publish":10,"buys":2,"sells":23}}}}}`

// webDePrueba is a fake es.wallapop.com: item pages carry the current build
// in their HTML, and data URLs only answer for that build. It counts the
// requests of each kind.
type webDePrueba struct {
	build, datos string
	estadoPagina int
	paginas      int
	pedidos      int
}

func (f *webDePrueba) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/item/"):
		f.paginas++
		if f.estadoPagina != 0 {
			w.WriteHeader(f.estadoPagina)
			return
		}
		fmt.Fprintf(w, `<script id="__NEXT_DATA__">{"buildId":"%s"}</script>`, f.build)
	case r.URL.Path == "/_next/data/"+f.build+"/es/item/iphone-13-blanco-nuevo-1326730384.json":
		f.pedidos++
		_, _ = w.Write([]byte(f.datos))
	default:
		f.pedidos++
		http.NotFound(w, r)
	}
}

func fichaEnWeb(t *testing.T, datos string) *webDePrueba {
	t.Helper()
	web := &webDePrueba{build: "b1", datos: datos}
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) { t.Error("the API is not used for listings") }, web.ServeHTTP)
	return web
}

const rutaFicha = "/item/iphone-13-blanco-nuevo-1326730384"

func TestAListingShowsItsData(t *testing.T) {
	fichaEnWeb(t, fichaDePrueba)
	w := pedirRuta(t, rutaFicha)
	html := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	for _, texto := range []string{
		"<title>iPhone 13 Blanco | Nuevo · wowpop</title>",
		"290 €",
		"Tecnología y electrónica › Smartphones",
		"iPhone 13 Blanco | 128GB\n\n&lt;b&gt;Batería&lt;/b&gt; al 100%",
		"<dt>Estado</dt><dd>En su caja</dd>",
		"<dt>Capacidad de almacenamiento</dt><dd>128 GB</dd>",
		"Móstoles · Con envío",
		"Actualizado el 21/11/2026 · 161 visitas · 17 favoritos",
		`chapa-destacado">Destacado`, `chapa-envio">Envío`,
		`id="ver-1" aria-label="Foto 1 de 2" checked>`, `src="/foto/images/10420/lx/wf/__/c1/i1.jpg?ancho=W800"`,
		`href="/foto/images/10420/lx/wf/__/c1/i1.jpg?ancho=W1024" target="_blank" rel="noopener"`,
		`fetchpriority="high"`,
		`<label for="ver-2" title="Foto 2">`, `src="/foto/images/10420/lx/wf/__/c1/i2.jpg?ancho=W320"`,
		`1 / 2`,
		`href="/user/chollop-474861123"`, "PhoneLab Store V.", "Profesional", "100 valoraciones · 23 ventas",
		`src="/foto/images/13/7u/px/__/c13p474861123/i3621427126.jpg?ancho=W320"`,
		`href="https://es.wallapop.com/item/iphone-13-blanco-nuevo-1326730384" rel="noreferrer noopener"`,
	} {
		if !strings.Contains(html, texto) {
			t.Errorf("missing %q", texto)
		}
	}
	if strings.Count(html, `class="galeria-foto"`) != 2 {
		t.Error("photos from other hosts must be left out")
	}
}

func TestAListingNeverShowsPrivateData(t *testing.T) {
	fichaEnWeb(t, fichaDePrueba)
	html := pedirRuta(t, rutaFicha).Body.String()
	for _, prohibido := range []string{"28930", "40.32", "-3.87", "ejk4rqmr79zx", "9jdworlgn96k", "evil.example", "cdn.wallapop.com", "1767869956"} {
		if strings.Contains(html, prohibido) {
			t.Errorf("the page contains %q", prohibido)
		}
	}
	if strings.Count(html, "wallapop.com") != 1 {
		t.Error("the only mention of wallapop.com is the link to buy there")
	}
}

func TestTheBuildIsReadOnceAndRefreshedWhenItChanges(t *testing.T) {
	web := fichaEnWeb(t, fichaDePrueba)
	for i := 0; i < 3; i++ {
		if w := pedirRuta(t, rutaFicha); w.Code != http.StatusOK {
			t.Fatalf("visit %d: %d", i, w.Code)
		}
	}
	if web.paginas != 1 || web.pedidos != 3 {
		t.Fatalf("three visits: %d pages and %d data requests, want 1 and 3", web.paginas, web.pedidos)
	}
	web.build = "b2"
	if w := pedirRuta(t, rutaFicha); w.Code != http.StatusOK {
		t.Fatalf("after a new build: %d", w.Code)
	}
	if web.paginas != 2 || web.pedidos != 5 {
		t.Errorf("a new build costs one page and one retry: %d pages, %d data requests", web.paginas, web.pedidos)
	}
}

func TestMissingListingsAre404(t *testing.T) {
	web := fichaEnWeb(t, fichaDePrueba)
	web.estadoPagina = http.StatusNotFound
	w := pedirRuta(t, rutaFicha)
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "Anuncio no encontrado") {
		t.Fatalf("got %d", w.Code)
	}
}

func TestBadSlugsNeverReachWallapop(t *testing.T) {
	web := fichaEnWeb(t, fichaDePrueba)
	for _, ruta := range []string{"/item/", "/item/a%2Fb", "/item/a.b", "/item/" + strings.Repeat("a", 201), "/item/caf%C3%A9"} {
		if w := pedirRuta(t, ruta); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", ruta, w.Code)
		}
	}
	if web.paginas+web.pedidos != 0 {
		t.Errorf("%d requests reached Wallapop", web.paginas+web.pedidos)
	}
}

func TestSoldListingsSaySo(t *testing.T) {
	fichaEnWeb(t, strings.Replace(fichaDePrueba, `"sold":false`, `"sold":true`, 1))
	html := pedirRuta(t, rutaFicha).Body.String()
	if !strings.Contains(html, `chapa-vendido">Vendido`) || !strings.Contains(html, "galeria-apagada") {
		t.Error("a sold listing must say so")
	}
}

func TestListingWallapopErrorsAre502(t *testing.T) {
	fichaEnWeb(t, "not json")
	if w := pedirRuta(t, rutaFicha); w.Code != http.StatusBadGateway {
		t.Fatalf("got %d", w.Code)
	}
}

func TestAListingWithoutDataIsGone(t *testing.T) {
	fichaEnWeb(t, `{"pageProps":{}}`)
	if w := pedirRuta(t, rutaFicha); w.Code != http.StatusNotFound {
		t.Fatalf("got %d", w.Code)
	}
}

func TestASellerWithoutAvatarGetsTheirInitial(t *testing.T) {
	sinAvatar := strings.Replace(fichaDePrueba, `"avatarImage":"https://cdn.wallapop.com/images/13/7u/px/__/c13p474861123/i3621427126.jpg?pictureSize=W320",`, `"microName2":"",`, 1)
	sinAvatar = strings.Replace(sinAvatar, `"microName":"PhoneLab Store V."`, `"microName":"ñandú"`, 1)
	fichaEnWeb(t, sinAvatar)
	if html := pedirRuta(t, rutaFicha).Body.String(); !strings.Contains(html, `vendedor-inicial" aria-hidden="true">Ñ<`) {
		t.Error("no initial")
	}
}

func TestDatesAreDayMonthYearInSpain(t *testing.T) {
	for ms, want := range map[int64]string{
		1795274769000: "21/11/2026",
		1767221999000: "31/12/2025", // 22:59:59 UTC on New Year's Eve
		1767222000000: "1/1/2026",   // midnight in Madrid, still 2025 in UTC
		0:             "",
	} {
		if got := formatearFecha(ms); got != want {
			t.Errorf("%d: %q, want %q", ms, got, want)
		}
	}
}

func TestTheGalleryNeverChangesTheAddress(t *testing.T) {
	fichaEnWeb(t, fichaDePrueba)
	html := pedirRuta(t, rutaFicha).Body.String()
	if strings.Contains(html, `href="#`) || strings.Contains(html, `id="foto-`) {
		t.Error("photos must not be anchors: changing photo would change the address and scroll the page")
	}
	if strings.Count(html, " checked>") != 1 {
		t.Error("exactly the first photo is shown at first")
	}
}

func TestTheArrowsWrapAround(t *testing.T) {
	f := nuevaFicha(fichaWallapop{Images: make([]struct {
		URLs struct {
			Small string `json:"small"`
			Big   string `json:"big"`
		} `json:"urls"`
	}, 3)}, vendedorWallapop{}, "x")
	if len(f.Fotos) != 0 {
		t.Fatal("photos without URLs are left out")
	}
	var a fichaWallapop
	for i := 1; i <= 3; i++ {
		var img struct {
			URLs struct {
				Small string `json:"small"`
				Big   string `json:"big"`
			} `json:"urls"`
		}
		img.URLs.Small = fmt.Sprintf("https://cdn.wallapop.com/images/a/%d.jpg", i)
		img.URLs.Big = img.URLs.Small
		a.Images = append(a.Images, img)
	}
	f = nuevaFicha(a, vendedorWallapop{}, "x")
	for i, want := range [][2]int{{3, 2}, {1, 3}, {2, 1}} {
		if p := f.Fotos[i]; p.Anterior != want[0] || p.Siguiente != want[1] || p.Total != 3 {
			t.Errorf("photo %d: previous %d, next %d, total %d", i+1, p.Anterior, p.Siguiente, p.Total)
		}
	}
}

func TestGalleryPhotosAreNotLazy(t *testing.T) {
	fichaEnWeb(t, fichaDePrueba)
	html := pedirRuta(t, rutaFicha).Body.String()
	i := strings.Index(html, `class="galeria`)
	j := strings.Index(html, `class="miniaturas"`)
	if i < 0 || j < i || strings.Contains(html[i:j], `loading="lazy"`) {
		t.Error("a hidden lazy photo only starts loading when shown, and its alt text flashes")
	}
}
