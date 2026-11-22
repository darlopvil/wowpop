package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Photos come from Wallapop's image server through wowpop, so the browser
// never contacts it. Pages link them as /foto/<path on the server>?ancho=W320
// and wowpop fetches the same path from cdn.wallapop.com. Only that host,
// only image paths and only the widths the website uses are accepted, so
// /foto/ cannot be used to reach anything else.

// hostFotos is where the photo URLs in Wallapop's answers point.
const hostFotos = "cdn.wallapop.com"

// limiteFoto is far above the biggest photo the website shows (W800, about
// 150 KB).
const limiteFoto = 4 << 20

var (
	reRutaFoto    = regexp.MustCompile(`^images/[A-Za-z0-9_/-]{1,200}\.(jpg|jpeg|png|webp)$`)
	anchosFoto    = map[string]bool{"W320": true, "W640": true, "W800": true}
	cabecerasFoto = []string{"Content-Type", "Cache-Control", "ETag", "Last-Modified", "Expires"}
)

// rutaFoto turns a photo URL from Wallapop's answers into a wowpop link at
// the given width, or "" if it does not point to the image server.
func rutaFoto(original, ancho string) string {
	u, err := url.Parse(original)
	if err != nil || u.Scheme != "https" || u.Host != hostFotos {
		return ""
	}
	ruta := strings.TrimPrefix(u.Path, "/")
	if !reRutaFoto.MatchString(ruta) || !anchosFoto[ancho] {
		return ""
	}
	return "/foto/" + ruta + "?ancho=" + ancho
}

func paginaFoto(w http.ResponseWriter, r *http.Request) {
	ruta := strings.TrimPrefix(r.URL.Path, "/foto/")
	ancho := r.URL.Query().Get("ancho")
	if !reRutaFoto.MatchString(ruta) || !anchosFoto[ancho] {
		http.NotFound(w, r)
		return
	}

	ctx, cancelar := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancelar()
	estado, cabeceras, cuerpo, err := wallapop.pedirFoto(ctx, wallapop.urlFoto(ruta, ancho))
	switch {
	case err != nil:
		log.Printf("photo: %v", err)
		http.Error(w, "No se pudo cargar la foto", http.StatusBadGateway)
		return
	case estado == http.StatusNotFound:
		http.NotFound(w, r)
		return
	case estado != http.StatusOK || !strings.HasPrefix(cabeceras.Get("Content-Type"), "image/"):
		log.Printf("photo: status %d, type %q", estado, cabeceras.Get("Content-Type"))
		http.Error(w, "No se pudo cargar la foto", http.StatusBadGateway)
		return
	}

	// The browser may keep a photo as long as the image server says (a
	// year), which saves asking wowpop, and Wallapop, again.
	c := w.Header()
	for _, nombre := range cabecerasFoto {
		if v := cabeceras.Get(nombre); v != "" {
			c.Set(nombre, v)
		}
	}
	if cabeceras.Get("Cache-Control") == "" {
		c.Set("Cache-Control", "public, max-age=86400")
	}
	_, _ = w.Write(cuerpo)
}

// pedirFoto fetches a photo from the image server. It asks for it
// uncompressed: JPEG gains nothing from gzip, and the body is passed on as
// it comes.
func (c *clienteWallapop) pedirFoto(ctx context.Context, direccion string) (int, http.Header, []byte, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, direccion, nil)
	if err != nil {
		return 0, nil, nil, err
	}
	r.Header.Set("User-Agent", agenteUsuario)
	r.Header.Set("Accept", "image/avif,image/webp,image/*,*/*;q=0.8")
	r.Header.Set("Accept-Language", idioma)
	r.Header.Set("Accept-Encoding", "identity")
	res, err := c.http.Do(r)
	if err != nil {
		return 0, nil, nil, err
	}
	defer res.Body.Close()
	cuerpo, err := io.ReadAll(io.LimitReader(res.Body, limiteFoto+1))
	if err != nil {
		return 0, nil, nil, err
	}
	if len(cuerpo) > limiteFoto {
		return 0, nil, nil, fmt.Errorf("photo over %d bytes", limiteFoto)
	}
	return res.StatusCode, res.Header, cuerpo, nil
}
