package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestEveryResponseCarriesTheSecurityHeaders(t *testing.T) {
	for _, ruta := range []string{"/", "/about", "/robots.txt", "/no-existe", estatico("estilo.css")} {
		h := pedirRuta(t, ruta).Header()
		for cabecera, valor := range map[string]string{
			"Content-Security-Policy": politicaContenido,
			"Referrer-Policy":         "no-referrer",
			"X-Content-Type-Options":  "nosniff",
			"X-Robots-Tag":            "noindex, nofollow",
		} {
			if h.Get(cabecera) != valor {
				t.Errorf("%s: %s = %q", ruta, cabecera, h.Get(cabecera))
			}
		}
	}
}

func TestThePolicyAllowsNoScriptsAndNoThirdParties(t *testing.T) {
	for _, parte := range []string{"default-src 'none'", "script-src 'none'", "frame-ancestors 'none'", "form-action 'self'"} {
		if !strings.Contains(politicaContenido, parte) {
			t.Errorf("policy lacks %q", parte)
		}
	}
	if strings.Contains(politicaContenido, "http") || strings.Contains(politicaContenido, "unsafe") {
		t.Error("the policy should only allow this same server")
	}
}

func TestPagesAreRevalidated(t *testing.T) {
	for _, ruta := range []string{"/", "/about"} {
		if c := pedirRuta(t, ruta).Header().Get("Cache-Control"); c != "no-cache" {
			t.Errorf("%s: Cache-Control %q", ruta, c)
		}
	}
}

func TestRobotsBlocksEveryCrawler(t *testing.T) {
	w := pedirRuta(t, "/robots.txt")
	if w.Code != http.StatusOK || w.Body.String() != "User-agent: *\nDisallow: /\n" {
		t.Fatalf("got %d: %q", w.Code, w.Body.String())
	}
}
