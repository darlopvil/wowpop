package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// fotosDePrueba points the image server at a fake one for the test.
func fotosDePrueba(t *testing.T, cdn http.HandlerFunc) {
	t.Helper()
	s := httptest.NewServer(cdn)
	anterior := wallapop.cdn
	wallapop.cdn = s.URL
	t.Cleanup(func() {
		wallapop.cdn = anterior
		s.Close()
	})
}

func TestPhotoLinksOnlyComeFromTheImageServer(t *testing.T) {
	for original, want := range map[string]string{
		"https://cdn.wallapop.com/images/10420/lx/zr/__/c10420p1326885872/i6880198806.jpg?pictureSize=W320": "/foto/images/10420/lx/zr/__/c10420p1326885872/i6880198806.jpg?ancho=W320",
		"https://cdn.wallapop.com/images/a/b.webp":                                                          "/foto/images/a/b.webp?ancho=W320",
		"http://cdn.wallapop.com/images/a/b.jpg":                                                            "",
		"https://evil.example/images/a/b.jpg":                                                               "",
		"https://cdn.wallapop.com/images/../etc/passwd":                                                     "",
		"https://cdn.wallapop.com/images/a/b.svg":                                                           "",
		"https://cdn.wallapop.com.evil.example/images/a.jpg":                                                "",
		"": "",
	} {
		if got := rutaFoto(original, "W320"); got != want {
			t.Errorf("%q: %q, want %q", original, got, want)
		}
	}
	if rutaFoto("https://cdn.wallapop.com/images/a/b.jpg", "W9999") != "" {
		t.Error("unknown widths are refused")
	}
}

func TestPhotosArePassedOnWithTheServersCaching(t *testing.T) {
	var recibida *http.Request
	fotosDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		recibida = r
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "max-age=31622400")
		w.Header().Set("ETag", `"abc"`)
		w.Header().Set("Set-Cookie", "rastreo=1")
		_, _ = w.Write([]byte("JPEG"))
	})
	w := pedirRuta(t, "/foto/images/10420/lx/zr/__/c1/i2.jpg?ancho=W640")
	if w.Code != http.StatusOK || w.Body.String() != "JPEG" {
		t.Fatalf("got %d %q", w.Code, w.Body.String())
	}
	if recibida.URL.Path != "/images/10420/lx/zr/__/c1/i2.jpg" || recibida.URL.Query().Get("pictureSize") != "W640" {
		t.Errorf("asked the server for %s", recibida.URL)
	}
	if recibida.Header.Get("Accept-Encoding") != "identity" || recibida.Header.Get("Cookie") != "" || recibida.Header.Get("Referer") != "" {
		t.Errorf("request headers: %v", recibida.Header)
	}
	for cabecera, valor := range map[string]string{"Content-Type": "image/jpeg", "Cache-Control": "max-age=31622400", "ETag": `"abc"`, "Set-Cookie": ""} {
		if w.Header().Get(cabecera) != valor {
			t.Errorf("%s = %q, want %q", cabecera, w.Header().Get(cabecera), valor)
		}
	}
}

func TestOnlyImagePathsAndKnownWidthsAreFetched(t *testing.T) {
	llamadas := 0
	fotosDePrueba(t, func(w http.ResponseWriter, r *http.Request) { llamadas++ })
	for _, ruta := range []string{
		"/foto/images/a/b.jpg",
		"/foto/images/a/b.jpg?ancho=W9999",
		"/foto/images/a/../b.jpg?ancho=W320",
		"/foto/static/a.jpg?ancho=W320",
		"/foto/images/a/b.html?ancho=W320",
		"/foto/?ancho=W320",
	} {
		// A path with ".." gets a redirect to its clean form from the router
		// (301 or 307 depending on the Go version).
		if w := pedirRuta(t, ruta); w.Code != http.StatusNotFound && (w.Code < 300 || w.Code > 399) {
			t.Errorf("%s: %d", ruta, w.Code)
		}
	}
	if llamadas != 0 {
		t.Errorf("%d requests reached the image server", llamadas)
	}
}

func TestWhatIsNotAnImageIsNotPassedOn(t *testing.T) {
	fotosDePrueba(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<script>"))
	})
	if w := pedirRuta(t, "/foto/images/a/b.jpg?ancho=W320"); w.Code != http.StatusBadGateway {
		t.Fatalf("got %d", w.Code)
	}
}

func TestMissingPhotosAre404(t *testing.T) {
	fotosDePrueba(t, http.NotFound)
	if w := pedirRuta(t, "/foto/images/a/b.jpg?ancho=W320"); w.Code != http.StatusNotFound {
		t.Fatalf("got %d", w.Code)
	}
}
