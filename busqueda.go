package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// /search?keywords=…[&orden=…][&b=…&p=…] shows a page of results as cards.
// Without b it is a new search; with it, page p of a search wowpop remembers
// (see paginacion.go).

// limitePalabras is well above any real search; longer ones are refused
// rather than cut, so the results always match what was typed.
const limitePalabras = 120

// ordenes are the orders offered, in the website's order_by values. Nearest
// first is left out until the location can be chosen.
var ordenes = []struct{ clave, nombre, api string }{
	{"", "Relevancia", "most_relevance"},
	{"nuevos", "Más nuevos", "newest"},
	{"baratos", "Más baratos", "price_low_to_high"},
	{"caros", "Más caros", "price_high_to_low"},
}

// reToken is the shape of meta.next_page: a signed JWT, a few hundred
// characters long. Anything else is never sent back to Wallapop.
var reToken = regexp.MustCompile(`^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)

// reClave is the shape of the key of a remembered search.
var reClave = regexp.MustCompile(`^[0-9a-f]{16}$`)

const limiteToken = 4096

func tokenValido(t string) bool {
	return len(t) <= limiteToken && reToken.MatchString(t)
}

// What wowpop reads from a search answer. Fields it does not show, such as
// the seller's coordinates, are not even decoded.
type respuestaBusqueda struct {
	Data struct {
		Section struct {
			Items []articuloBusqueda `json:"items"`
		} `json:"section"`
	} `json:"data"`
	Meta struct {
		NextPage string `json:"next_page"`
	} `json:"meta"`
}

type bandera struct {
	Flag bool `json:"flag"`
}

type articuloBusqueda struct {
	Title   string `json:"title"`
	WebSlug string `json:"web_slug"`
	Price   struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	} `json:"price"`
	Images []struct {
		URLs struct {
			Small string `json:"small"`
		} `json:"urls"`
	} `json:"images"`
	Reserved bandera `json:"reserved"`
	Shipping struct {
		ItemIsShippable    bool `json:"item_is_shippable"`
		UserAllowsShipping bool `json:"user_allows_shipping"`
	} `json:"shipping"`
	HasWarranty bandera `json:"has_warranty"`
	Bump        struct {
		Type string `json:"type"`
	} `json:"bump"`
	Location struct {
		City string `json:"city"`
	} `json:"location"`
}

// resultadosBusqueda is what the search template shows.
type resultadosBusqueda struct {
	Palabras  string
	Ordenes   []opcionOrden
	Articulos []tarjeta
	// Paginas are the numbered page links, shown once there is more than
	// one; Anterior and Siguiente are empty on the first and last pages.
	Paginas             []enlacePagina
	Anterior, Siguiente string
}

type enlacePagina struct {
	Numero int
	Enlace string
	// Activa is the page being shown; Hueco stands for the pages left out
	// between distant numbers.
	Activa, Hueco bool
}

type opcionOrden struct {
	Nombre, Enlace string
	Activa         bool
}

type tarjeta struct {
	Titulo, Enlace, Foto, Precio, Ciudad string
	// Destacado means the seller paid to push the listing up the results:
	// it is shown, so nobody takes it for the most relevant one.
	Reservado, Envio, Garantia, Destacado bool
}

func paginaBusqueda(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	palabras := strings.Join(strings.Fields(q.Get("keywords")), " ")
	if palabras == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if utf8.RuneCountInString(palabras) > limitePalabras {
		renderizar(w, http.StatusBadRequest, "error", datosPagina{
			Titulo:   "Búsqueda demasiado larga",
			Mensaje:  "Prueba con menos palabras.",
			Consulta: palabras,
		})
		return
	}
	clave, orden := q.Get("orden"), ordenes[0]
	for _, o := range ordenes {
		if o.clave == clave {
			orden = o
		}
	}

	// A known search opens the page asked for; anything else is a new
	// search, from its first page.
	clave, n := q.Get("b"), 1
	if v, err := strconv.Atoi(q.Get("p")); err == nil {
		n = v
	}
	token, id, conocida := "", "", false
	if reClave.MatchString(clave) {
		token, id, _, conocida = busquedas.pagina(clave, palabras, orden.clave, n)
	}
	if !conocida {
		if clave != "" && n > 1 {
			resultadosCaducados(w, palabras, orden.clave)
			return
		}
		n, id = 1, nuevoUUID()
		clave = busquedas.crear(palabras, orden.clave, id)
	}
	direccion := wallapop.urlBusquedaID(palabras, orden.api, id, latitudPorDefecto, longitudPorDefecto)
	if n > 1 {
		direccion = wallapop.urlSiguiente(token)
	}

	ctx, cancelar := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancelar()
	res, err := wallapop.pedirJSON(ctx, direccion)
	var datos respuestaBusqueda
	if err == nil && res.Estado == http.StatusOK {
		err = json.Unmarshal(res.Cuerpo, &datos)
	}
	switch {
	case err != nil:
		log.Printf("search: %v", err)
		errorWallapop(w, palabras)
		return
	case n > 1 && res.Estado >= 400 && res.Estado < 500:
		// The usual reason is a token older than a day.
		resultadosCaducados(w, palabras, orden.clave)
		return
	case res.Estado != http.StatusOK:
		log.Printf("search: Wallapop answered %d", res.Estado)
		errorWallapop(w, palabras)
		return
	}

	vista := &resultadosBusqueda{Palabras: palabras}
	for _, o := range ordenes {
		vista.Ordenes = append(vista.Ordenes, opcionOrden{
			Nombre: o.nombre,
			Enlace: enlaceBusqueda(palabras, o.clave, "", 0),
			Activa: o.clave == orden.clave,
		})
	}
	for _, a := range datos.Data.Section.Items {
		vista.Articulos = append(vista.Articulos, nuevaTarjeta(a))
	}
	total := busquedas.conocidas(clave)
	if t := datos.Meta.NextPage; t != "" && len(vista.Articulos) > 0 && tokenValido(t) {
		total = busquedas.anotar(clave, n, t)
	}
	paginar(vista, palabras, orden.clave, clave, n, total)

	titulo := "«" + palabras + "»"
	if n > 1 {
		titulo += " · página " + strconv.Itoa(n)
	}
	renderizar(w, http.StatusOK, "busqueda", datosPagina{
		Titulo:     titulo,
		Consulta:   palabras,
		Resultados: vista,
	})
}

// paginar fills in the page links: always the first and the last known
// pages, and two on each side of the current one.
func paginar(v *resultadosBusqueda, palabras, orden, clave string, n, total int) {
	if total <= 1 {
		return
	}
	if n > 1 {
		v.Anterior = enlaceBusqueda(palabras, orden, clave, n-1)
	}
	if n < total {
		v.Siguiente = enlaceBusqueda(palabras, orden, clave, n+1)
	}
	for i := 1; i <= total; i++ {
		switch {
		case i == 1 || i == total || (i >= n-2 && i <= n+2):
			v.Paginas = append(v.Paginas, enlacePagina{Numero: i, Enlace: enlaceBusqueda(palabras, orden, clave, i), Activa: i == n})
		case len(v.Paginas) > 0 && !v.Paginas[len(v.Paginas)-1].Hueco:
			v.Paginas = append(v.Paginas, enlacePagina{Hueco: true})
		}
	}
}

func resultadosCaducados(w http.ResponseWriter, palabras, orden string) {
	renderizar(w, http.StatusGone, "error", datosPagina{
		Titulo:   "Resultados caducados",
		Mensaje:  "Esta página de resultados ha caducado. Vuelve a buscar para verla al día.",
		Consulta: palabras,
		Volver:   enlaceBusqueda(palabras, orden, "", 0),
	})
}

func errorWallapop(w http.ResponseWriter, palabras string) {
	renderizar(w, http.StatusBadGateway, "error", datosPagina{
		Titulo:   "Wallapop no responde",
		Mensaje:  "No se ha podido buscar en Wallapop ahora mismo. Prueba de nuevo en un rato.",
		Consulta: palabras,
	})
}

// enlaceBusqueda links a search in an order and, with a key, one of its
// pages; without a key, a new search.
func enlaceBusqueda(palabras, orden, clave string, n int) string {
	q := url.Values{"keywords": {palabras}}
	if orden != "" {
		q.Set("orden", orden)
	}
	if clave != "" {
		q.Set("b", clave)
		q.Set("p", strconv.Itoa(n))
	}
	return "/search?" + q.Encode()
}

func nuevaTarjeta(a articuloBusqueda) tarjeta {
	t := tarjeta{
		Titulo:    a.Title,
		Precio:    formatearPrecio(a.Price.Amount, a.Price.Currency),
		Ciudad:    a.Location.City,
		Reservado: a.Reserved.Flag,
		Envio:     a.Shipping.ItemIsShippable && a.Shipping.UserAllowsShipping,
		Garantia:  a.HasWarranty.Flag,
		Destacado: a.Bump.Type != "" && a.Bump.Type != "none",
	}
	if a.WebSlug != "" {
		t.Enlace = "/item/" + url.PathEscape(a.WebSlug)
	}
	if len(a.Images) > 0 {
		t.Foto = rutaFoto(a.Images[0].URLs.Small, "W320")
	}
	return t
}

// formatearPrecio writes a price the Spanish way: 1.250 €, 12,99 €. Whole
// amounts go without decimals, as on the website.
func formatearPrecio(cantidad float64, moneda string) string {
	if math.IsNaN(cantidad) || math.IsInf(cantidad, 0) || cantidad < 0 {
		return ""
	}
	centimos := int64(math.Round(cantidad * 100))
	entero := strconv.FormatInt(centimos/100, 10)
	for i := len(entero) - 3; i > 0; i -= 3 {
		entero = entero[:i] + "." + entero[i:]
	}
	if c := centimos % 100; c != 0 {
		entero += "," + strconv.FormatInt(100+c, 10)[1:]
	}
	simbolo := moneda
	if moneda == "EUR" || moneda == "" {
		simbolo = "€"
	}
	return entero + "\u00a0" + simbolo
}
