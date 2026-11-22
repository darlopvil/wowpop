package main

import "net/http"

// politicaContenido only lets pages use what this same server sends: its
// stylesheet, fonts and images. No scripts at all, no third parties, no
// framing, and forms that can only submit here.
const politicaContenido = "default-src 'none'; script-src 'none'; style-src 'self'; img-src 'self'; font-src 'self'; " +
	"form-action 'self'; base-uri 'none'; frame-ancestors 'none'"

// cabeceras adds the headers every response carries. Handlers that know
// better (static files, photos) replace Cache-Control with their own.
func cabeceras(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := w.Header()
		// This is a private instance: search engines must not index it.
		c.Set("X-Robots-Tag", "noindex, nofollow")
		c.Set("Cache-Control", "no-cache")
		c.Set("Content-Security-Policy", politicaContenido)
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("X-Content-Type-Options", "nosniff")
		h.ServeHTTP(w, r)
	})
}

// robots blocks every crawler, so none of them spends requests to Wallapop.
func robots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
}
