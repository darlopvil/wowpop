package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// wallapopDePrueba points the client at two fake servers, one for the API
// and one for the website, for the length of the test.
func wallapopDePrueba(t *testing.T, api, web http.HandlerFunc) {
	t.Helper()
	sa := httptest.NewServer(api)
	sw := httptest.NewServer(web)
	anterior := wallapop
	wallapop = nuevoCliente(sa.URL, sw.URL)
	t.Cleanup(func() {
		wallapop = anterior
		sa.Close()
		sw.Close()
	})
}

func TestTheAPIGetsTheBrowserHeadersAndXDeviceOS(t *testing.T) {
	var recibida http.Header
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		recibida = r.Header.Clone()
		_, _ = w.Write([]byte(`{}`))
	}, http.NotFound)

	if _, err := wallapop.pedirJSON(context.Background(), wallapop.urlBusqueda("gameboy", "most_relevance", 1, 2)); err != nil {
		t.Fatal(err)
	}
	for cabecera, valor := range map[string]string{
		"User-Agent":      agenteUsuario,
		"Accept":          "application/json",
		"Accept-Language": idioma,
		"X-Deviceos":      "0",
	} {
		if recibida.Get(cabecera) != valor {
			t.Errorf("%s = %q", cabecera, recibida.Get(cabecera))
		}
	}
	for _, cabecera := range []string{"Cookie", "X-Deviceid", "X-Appversion", "Deviceos", "Authorization"} {
		if recibida.Get(cabecera) != "" {
			t.Errorf("%s should not be sent", cabecera)
		}
	}
}

func TestTheWebsiteDoesNotGetXDeviceOS(t *testing.T) {
	var recibida http.Header
	wallapopDePrueba(t, http.NotFound, func(w http.ResponseWriter, r *http.Request) {
		recibida = r.Header.Clone()
		_, _ = w.Write([]byte(`<html></html>`))
	})
	if _, err := wallapop.pedirHTML(context.Background(), wallapop.urlPagina("item", "algo-123")); err != nil {
		t.Fatal(err)
	}
	if recibida.Get("X-Deviceos") != "" || !strings.HasPrefix(recibida.Get("Accept"), "text/html") {
		t.Errorf("headers: %v", recibida)
	}
}

func TestErrorStatusesAreReturnedNotHidden(t *testing.T) {
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}, http.NotFound)
	r, err := wallapop.pedirJSON(context.Background(), wallapop.urlArticulosVendedor("x", ""))
	if err != nil || r.Estado != http.StatusForbidden {
		t.Fatalf("got %d, %v", r.Estado, err)
	}
}

func TestOversizedResponsesAreRefused(t *testing.T) {
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, limiteRespuesta+1))
	}, http.NotFound)
	if _, err := wallapop.pedirJSON(context.Background(), wallapop.urlArticulosVendedor("x", "")); err == nil {
		t.Fatal("a response over the limit should be an error")
	}
}

func TestRedirectsOutsideWallapopAreRefused(t *testing.T) {
	fuera := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the request reached a host outside Wallapop")
	}))
	defer fuera.Close()
	wallapopDePrueba(t, http.NotFound, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, fuera.URL+"/x", http.StatusFound)
	})
	if _, err := wallapop.pedirHTML(context.Background(), wallapop.urlPagina("user", "alguien-1")); err == nil {
		t.Fatal("a redirect to another host should be an error")
	}
}

func TestRedirectsWithinWallapopAreFollowed(t *testing.T) {
	var web string
	wallapopDePrueba(t, http.NotFound, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/item/viejo-1" {
			http.Redirect(w, r, web+"/item/nuevo-1", http.StatusMovedPermanently)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	web = wallapop.web
	r, err := wallapop.pedirHTML(context.Background(), wallapop.urlPagina("item", "viejo-1"))
	if err != nil || string(r.Cuerpo) != "ok" {
		t.Fatalf("got %q, %v", r.Cuerpo, err)
	}
}

func TestSearchAsksForOrganicResultsOnly(t *testing.T) {
	u, err := url.Parse(wallapop.urlBusqueda("tomb raider", "most_relevance", latitudPorDefecto, longitudPorDefecto))
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "api.wallapop.com" || u.Path != "/api/v3/search/section" {
		t.Fatalf("wrong endpoint: %s", u)
	}
	q := u.Query()
	for clave, valor := range map[string]string{
		"keywords":       "tomb raider",
		"source":         "search_box",
		"order_by":       "most_relevance",
		"latitude":       "40.4165",
		"longitude":      "-3.70256",
		"search_country": "ES",
		"search_section": "organic_search_results",
		"section_type":   "keyword_search",
	} {
		if q.Get(clave) != valor {
			t.Errorf("%s = %q", clave, q.Get(clave))
		}
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(q.Get("search_id")) {
		t.Errorf("search_id is not a version 4 UUID: %q", q.Get("search_id"))
	}
	if strings.Contains(u.RawQuery, "similar_items") || strings.Contains(u.RawQuery, "vector_search") {
		t.Error("the similar items filler must never be asked for")
	}
}

func TestEverySearchGetsItsOwnID(t *testing.T) {
	a, _ := url.Parse(wallapop.urlBusqueda("x", "newest", 0, 0))
	b, _ := url.Parse(wallapop.urlBusqueda("x", "newest", 0, 0))
	if a.Query().Get("search_id") == b.Query().Get("search_id") {
		t.Error("two searches share a search_id")
	}
}

func TestSellerItemsCarryTheCursorOnlyWhenThereIsOne(t *testing.T) {
	if got := wallapop.urlArticulosVendedor("lqzm9l5pdx6v", ""); got != "https://api.wallapop.com/api/v3/users/lqzm9l5pdx6v/items" {
		t.Errorf("first page: %s", got)
	}
	cursor := "SSMxMzI0ODEwOTkwX0RfVSMxNDg2NjQwOF9QIzIwMjYtMTEtMTVUMjE6MDk6MjIuMzQ4Wg=="
	want := "https://api.wallapop.com/api/v3/users/lqzm9l5pdx6v/items?since=SSMxMzI0ODEwOTkwX0RfVSMxNDg2NjQwOF9QIzIwMjYtMTEtMTVUMjE6MDk6MjIuMzQ4Wg%3D%3D"
	if got := wallapop.urlArticulosVendedor("lqzm9l5pdx6v", cursor); got != want {
		t.Errorf("next page: %s", got)
	}
}

func TestPageAndDataURLs(t *testing.T) {
	if got := wallapop.urlPagina("item", "tomb-raider-1326118210"); got != "https://es.wallapop.com/item/tomb-raider-1326118210" {
		t.Errorf("page: %s", got)
	}
	want := "https://es.wallapop.com/_next/data/mw3dDXPGEWki5yluEf-kn/es/user/yasiml-53114493.json"
	if got := wallapop.urlDatos("mw3dDXPGEWki5yluEf-kn", "user", "yasiml-53114493"); got != want {
		t.Errorf("data: %s", got)
	}
	if got := wallapop.urlPagina("item", "a/../b"); strings.Contains(got, "/../") {
		t.Errorf("a slug must not change the path: %s", got)
	}
}

func TestBuildIDComesFromNextData(t *testing.T) {
	html := []byte(`<script id="__NEXT_DATA__" type="application/json">{"props":{},"page":"/item/[itemSlug]","buildId":"mw3dDXPGEWki5yluEf-kn","isFallback":false}</script>`)
	if id, err := leerBuildID(html); err != nil || id != "mw3dDXPGEWki5yluEf-kn" {
		t.Fatalf("got %q, %v", id, err)
	}
	for _, malo := range []string{`<html></html>`, `"buildId":"../../etc"`, `"buildId":""`} {
		if _, err := leerBuildID([]byte(malo)); err == nil {
			t.Errorf("%s should not give a buildId", malo)
		}
	}
}
