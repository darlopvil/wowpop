package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Everything wowpop asks Wallapop goes through this file. It sends what the
// official website sends to a visitor without an account, minus anything
// that identifies the visitor: no cookies, no device id, no app version.

const (
	// Firefox on Linux, like the browser the requests were measured with.
	agenteUsuario = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0"
	idioma        = "es-ES,es;q=0.9"

	// The biggest response measured is a listing HTML page, about 176 KB.
	limiteRespuesta = 2 << 20
)

// Puerta del Sol, Madrid. It is what the website sends when "Madrid" is
// chosen as the location, and wowpop sends it until the user picks one, so
// Wallapop never locates anyone by IP unasked.
const (
	latitudPorDefecto  = 40.4165
	longitudPorDefecto = -3.70256
)

type clienteWallapop struct {
	http *http.Client
	// api serves searches, filters and a seller's items; web serves the
	// pages, whose data comes from Next.js (_next/data and __NEXT_DATA__).
	api, web string
}

var wallapop = nuevoCliente("https://api.wallapop.com", "https://es.wallapop.com")

func nuevoCliente(api, web string) *clienteWallapop {
	c := &clienteWallapop{api: strings.TrimSuffix(api, "/"), web: strings.TrimSuffix(web, "/")}
	permitidos := map[string]bool{hostDe(c.api): true, hostDe(c.web): true}
	c.http = &http.Client{
		Timeout: 15 * time.Second,
		// Redirects are followed only within Wallapop's own hosts.
		CheckRedirect: func(r *http.Request, previas []*http.Request) error {
			if len(previas) >= 3 {
				return errors.New("too many redirects")
			}
			if !permitidos[r.URL.Host] {
				return fmt.Errorf("redirect to %s refused", r.URL.Host)
			}
			return nil
		},
	}
	return c
}

func hostDe(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		panic(err) // fixed base URLs: only a broken build gets here
	}
	return u.Host
}

// respuesta is what came back from Wallapop.
type respuesta struct {
	Estado   int
	Cuerpo   []byte
	Duracion time.Duration
}

// pedir sends one GET to Wallapop and reads the whole body, refusing
// anything over limiteRespuesta. A non-2xx status is not an error here:
// the caller decides what it means.
func (c *clienteWallapop) pedir(ctx context.Context, direccion, acepta string) (respuesta, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, direccion, nil)
	if err != nil {
		return respuesta{}, err
	}
	r.Header.Set("User-Agent", agenteUsuario)
	r.Header.Set("Accept", acepta)
	r.Header.Set("Accept-Language", idioma)
	// The one header the API insists on: without it, every request is a 403.
	if strings.HasPrefix(direccion, c.api+"/") {
		r.Header.Set("X-DeviceOS", "0")
	}

	inicio := time.Now()
	res, err := c.http.Do(r)
	if err != nil {
		return respuesta{}, err
	}
	defer res.Body.Close()
	cuerpo, err := io.ReadAll(io.LimitReader(res.Body, limiteRespuesta+1))
	if err != nil {
		return respuesta{}, err
	}
	if len(cuerpo) > limiteRespuesta {
		return respuesta{}, fmt.Errorf("response over %d bytes", limiteRespuesta)
	}
	return respuesta{Estado: res.StatusCode, Cuerpo: cuerpo, Duracion: time.Since(inicio)}, nil
}

func (c *clienteWallapop) pedirJSON(ctx context.Context, direccion string) (respuesta, error) {
	return c.pedir(ctx, direccion, "application/json")
}

func (c *clienteWallapop) pedirHTML(ctx context.Context, direccion string) (respuesta, error) {
	return c.pedir(ctx, direccion, "text/html,application/xhtml+xml")
}

// urlBusqueda is a new search with the parameters the website sends. Only
// the organic results are asked for: never the "similar items" filler.
func (c *clienteWallapop) urlBusqueda(palabras string, latitud, longitud float64) string {
	q := url.Values{
		"keywords":       {palabras},
		"source":         {"search_box"},
		"order_by":       {"most_relevance"},
		"search_id":      {nuevoUUID()},
		"latitude":       {strconv.FormatFloat(latitud, 'f', -1, 64)},
		"longitude":      {strconv.FormatFloat(longitud, 'f', -1, 64)},
		"search_country": {"ES"},
		"search_section": {"organic_search_results"},
		"section_type":   {"keyword_search"},
	}
	return c.api + "/api/v3/search/section?" + q.Encode()
}

// urlArticulosVendedor is a page of a seller's items: the first one without
// cursor, the next ones with the meta.next of the previous page.
func (c *clienteWallapop) urlArticulosVendedor(id, cursor string) string {
	u := c.api + "/api/v3/users/" + url.PathEscape(id) + "/items"
	if cursor != "" {
		u += "?since=" + url.QueryEscape(cursor)
	}
	return u
}

// urlPagina is a Wallapop page, "item" or "user", as a browser opens it.
func (c *clienteWallapop) urlPagina(tipo, slug string) string {
	return c.web + "/" + tipo + "/" + url.PathEscape(slug)
}

// urlDatos is the data of that same page, as the website loads it when it
// moves between pages without a full reload. buildId changes with every
// deployment of the website.
func (c *clienteWallapop) urlDatos(buildID, tipo, slug string) string {
	return c.web + "/_next/data/" + url.PathEscape(buildID) + "/es/" + tipo + "/" + url.PathEscape(slug) + ".json"
}

var reBuildID = regexp.MustCompile(`"buildId":"([A-Za-z0-9_-]{1,64})"`)

// leerBuildID takes the buildId out of the __NEXT_DATA__ of a Wallapop page.
func leerBuildID(html []byte) (string, error) {
	m := reBuildID.FindSubmatch(html)
	if m == nil {
		return "", errors.New("no buildId in the page")
	}
	return string(m[1]), nil
}

// nuevoUUID is a random version 4 UUID, as the website makes one for every
// new search.
func nuevoUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
