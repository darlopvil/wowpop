package main

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestProbeSearchSummarisesTheResults(t *testing.T) {
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/search/section" || r.URL.Query().Get("keywords") != "gameboy" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"section":{"items":[{"title":"Game Boy Color"},{"title":"Pokémon Rojo"}]}},"meta":{"next_page":"eyJ"}}`))
	}, http.NotFound)

	var volcado, salida bytes.Buffer
	if codigo := sondear("busqueda=gameboy", true, &volcado, &salida); codigo != 0 {
		t.Fatalf("exit %d:\n%s", codigo, salida.String())
	}
	for _, texto := range []string{"search", "200", "items: 2", "first: Game Boy Color", "next_page: true"} {
		if !strings.Contains(salida.String(), texto) {
			t.Errorf("summary lacks %q:\n%s", texto, salida.String())
		}
	}
	if !strings.HasPrefix(volcado.String(), `{"data":`) {
		t.Errorf("raw dump: %q", volcado.String())
	}
}

func TestProbeSellerFollowsPageDataAndItems(t *testing.T) {
	const buildID = "abc123"
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/users/k3zl/items" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"1"},{"id":"2"}],"meta":{"next":"SSM"}}`))
	}, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user/yas-1":
			_, _ = w.Write([]byte(`<script id="__NEXT_DATA__">{"buildId":"` + buildID + `"}</script>`))
		case "/_next/data/" + buildID + "/es/user/yas-1.json":
			_, _ = w.Write([]byte(`{"pageProps":{"user":{"id":"k3zl","microName":"Yas .."},"publishedItems":{"data":[{}],"meta":{}}}}`))
		default:
			http.NotFound(w, r)
		}
	})

	var volcado, salida bytes.Buffer
	if codigo := sondear("vendedor=yas-1", false, &volcado, &salida); codigo != 0 {
		t.Fatalf("exit %d:\n%s", codigo, salida.String())
	}
	for _, texto := range []string{"buildId: " + buildID, "alias: Yas ..", "published: 1", "published.meta: {}", "items: 2", "meta.next: true"} {
		if !strings.Contains(salida.String(), texto) {
			t.Errorf("summary lacks %q:\n%s", texto, salida.String())
		}
	}
	if volcado.Len() != 0 {
		t.Error("without -crudo nothing should be dumped")
	}
}

func TestProbeFailsOnAnErrorStatus(t *testing.T) {
	wallapopDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "", http.StatusForbidden)
	}, http.NotFound)
	var volcado, salida bytes.Buffer
	if codigo := sondear("busqueda=x", false, &volcado, &salida); codigo != 1 || !strings.Contains(salida.String(), "403") {
		t.Fatalf("exit %d:\n%s", codigo, salida.String())
	}
}

func TestProbeRejectsBadOrders(t *testing.T) {
	for _, orden := range []string{"", "busqueda", "busqueda=", "otra=x"} {
		var volcado, salida bytes.Buffer
		if codigo := sondear(orden, false, &volcado, &salida); codigo != 2 {
			t.Errorf("%q: exit %d", orden, codigo)
		}
	}
}
