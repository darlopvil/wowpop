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
)

// /user/<slug>[?b=…&p=…] is a seller's profile and their items, at the same
// path as on es.wallapop.com. The profile comes from the website's data
// route (datosweb.go) and the items from the API, 40 at a time; each page
// of items comes with a cursor for the next, kept like a search's tokens
// (paginacion.go).

// reCursor is the shape of the API's meta.next: base64.
var reCursor = regexp.MustCompile(`^[A-Za-z0-9+/=_-]{1,512}$`)

// What wowpop reads from a profile's data. Its coordinates, postal code,
// registration date and gender are not decoded.
type datosVendedor struct {
	PageProps struct {
		User      perfilWallapop `json:"user"`
		UserStats statsWallapop  `json:"userStats"`
	} `json:"pageProps"`
}

type perfilWallapop struct {
	ID                    string `json:"id"`
	MicroName             string `json:"microName"`
	AvatarImage           string `json:"avatarImage"`
	SellerType            string `json:"sellerType"`
	Verified              bool   `json:"verified"`
	IsVacationModeEnabled bool   `json:"isVacationModeEnabled"`
	Location              struct {
		City string `json:"city"`
	} `json:"location"`
	ExtraInfoPro *struct {
		Description string `json:"description"`
	} `json:"extraInfoPro"`
}

// statsWallapop: ratings.reviews is a score out of 100 (5 stars are 100),
// while counters.reviews is how many reviews there are.
type statsWallapop struct {
	Ratings struct {
		Reviews float64 `json:"reviews"`
	} `json:"ratings"`
	Counters struct {
		Reviews int `json:"reviews"`
		Sells   int `json:"sells"`
		Buys    int `json:"buys"`
	} `json:"counters"`
}

// What wowpop reads from a page of a seller's items.
type respuestaVendedor struct {
	Data []articuloVendedor `json:"data"`
	Meta struct {
		Next string `json:"next"`
	} `json:"meta"`
}

type articuloVendedor struct {
	Title  string `json:"title"`
	Slug   string `json:"slug"`
	Images []struct {
		URLs struct {
			Small string `json:"small"`
		} `json:"urls"`
	} `json:"images"`
	Price struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	} `json:"price"`
	Bump struct {
		Type string `json:"type"`
	} `json:"bump"`
	Shipping struct {
		ItemIsShippable    bool `json:"item_is_shippable"`
		UserAllowsShipping bool `json:"user_allows_shipping"`
	} `json:"shipping"`
	HasWarranty bool `json:"has_warranty"`
}

// perfil is what the seller template shows.
type perfil struct {
	Alias, Avatar, Inicial, Ciudad, Descripcion string
	Profesional, Verificado, Vacaciones         bool
	// Nota is the rating out of 5, written the Spanish way; empty without
	// reviews.
	Nota                          string
	Valoraciones, Ventas, Compras int
	EnWallapop                    string
	Articulos                     []tarjeta
	Paginacion
}

func paginaVendedor(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/user/")
	if !reSlug.MatchString(slug) {
		vendedorNoEncontrado(w)
		return
	}

	ctx, cancelar := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancelar()
	res, err := wallapop.pedirDatosWeb(ctx, "user", slug)
	var datos datosVendedor
	if err == nil && res.Estado == http.StatusOK {
		err = json.Unmarshal(res.Cuerpo, &datos)
	}
	switch {
	case err != nil:
		log.Printf("seller: %v", err)
		errorVendedor(w)
		return
	case res.Estado == http.StatusNotFound || res.Estado == http.StatusGone:
		vendedorNoEncontrado(w)
		return
	case res.Estado != http.StatusOK:
		log.Printf("seller: Wallapop answered %d", res.Estado)
		errorVendedor(w)
		return
	}
	u := datos.PageProps.User
	if u.ID == "" || u.MicroName == "" {
		vendedorNoEncontrado(w)
		return
	}

	// The items: page 1 without cursor, later ones with the cursor kept
	// under the key in the links. The seller's id goes where a search keeps
	// its search_id.
	busqueda := "vendedor " + slug
	clave, n := r.URL.Query().Get("b"), 1
	if v, err := strconv.Atoi(r.URL.Query().Get("p")); err == nil {
		n = v
	}
	cursor, id, conocida := "", "", false
	if reClave.MatchString(clave) {
		cursor, id, _, conocida = busquedas.pagina(clave, busqueda, "", n)
	}
	if !conocida || id != u.ID {
		if clave != "" && n > 1 {
			articulosCaducados(w, slug)
			return
		}
		n = 1
		clave = busquedas.crear(busqueda, "", u.ID)
		cursor = ""
	}
	res, err = wallapop.pedirJSON(ctx, wallapop.urlArticulosVendedor(u.ID, cursor))
	var articulos respuestaVendedor
	if err == nil && res.Estado == http.StatusOK {
		err = json.Unmarshal(res.Cuerpo, &articulos)
	}
	switch {
	case err != nil:
		log.Printf("seller items: %v", err)
		errorVendedor(w)
		return
	case n > 1 && res.Estado >= 400 && res.Estado < 500:
		articulosCaducados(w, slug)
		return
	case res.Estado != http.StatusOK:
		log.Printf("seller items: Wallapop answered %d", res.Estado)
		errorVendedor(w)
		return
	}

	p := nuevoPerfil(u, datos.PageProps.UserStats, slug)
	for _, a := range articulos.Data {
		p.Articulos = append(p.Articulos, tarjetaVendedor(a))
	}
	total := busquedas.conocidas(clave)
	if c := articulos.Meta.Next; reCursor.MatchString(c) && len(articulos.Data) > 0 {
		total = busquedas.anotar(clave, n, c)
	}
	p.Paginacion = paginar(n, total, func(i int) string {
		return "/user/" + slug + "?" + url.Values{"b": {clave}, "p": {strconv.Itoa(i)}}.Encode()
	})

	titulo := p.Alias
	if n > 1 {
		titulo += " · página " + strconv.Itoa(n)
	}
	renderizar(w, http.StatusOK, "vendedor", datosPagina{Titulo: titulo, Perfil: p})
}

func vendedorNoEncontrado(w http.ResponseWriter) {
	renderizar(w, http.StatusNotFound, "error", datosPagina{
		Titulo:  "Perfil no encontrado",
		Mensaje: "Este perfil no existe o ya no está en Wallapop.",
	})
}

func errorVendedor(w http.ResponseWriter) {
	renderizar(w, http.StatusBadGateway, "error", datosPagina{
		Titulo:  "Wallapop no responde",
		Mensaje: "No se ha podido abrir el perfil ahora mismo. Prueba de nuevo en un rato.",
	})
}

func articulosCaducados(w http.ResponseWriter, slug string) {
	renderizar(w, http.StatusGone, "error", datosPagina{
		Titulo:  "Página caducada",
		Mensaje: "Esta página de artículos ha caducado. Vuelve al perfil para verla al día.",
		Volver:  "/user/" + slug,
	})
}

func nuevoPerfil(u perfilWallapop, s statsWallapop, slug string) *perfil {
	p := &perfil{
		Alias:        u.MicroName,
		Avatar:       rutaFoto(u.AvatarImage, "W320"),
		Ciudad:       u.Location.City,
		Profesional:  strings.EqualFold(u.SellerType, "pro"),
		Verificado:   u.Verified,
		Vacaciones:   u.IsVacationModeEnabled,
		Valoraciones: s.Counters.Reviews,
		Ventas:       s.Counters.Sells,
		Compras:      s.Counters.Buys,
		EnWallapop:   "https://es.wallapop.com/user/" + url.PathEscape(slug),
	}
	if u.ExtraInfoPro != nil {
		p.Descripcion = strings.TrimSpace(u.ExtraInfoPro.Description)
	}
	if p.Valoraciones > 0 {
		p.Nota = formatearNota(s.Ratings.Reviews)
	}
	if p.Avatar == "" {
		p.Inicial = inicial(u.MicroName)
	}
	return p
}

func tarjetaVendedor(a articuloVendedor) tarjeta {
	t := tarjeta{
		Titulo:    a.Title,
		Precio:    formatearPrecio(a.Price.Amount, a.Price.Currency),
		Envio:     a.Shipping.ItemIsShippable && a.Shipping.UserAllowsShipping,
		Garantia:  a.HasWarranty,
		Destacado: a.Bump.Type != "" && a.Bump.Type != "none",
	}
	if reSlug.MatchString(a.Slug) {
		t.Enlace = "/item/" + a.Slug
	}
	if len(a.Images) > 0 {
		t.Foto = rutaFoto(a.Images[0].URLs.Small, "W320")
	}
	return t
}

// formatearNota turns a score out of 100 into stars out of 5 with one
// decimal, the Spanish way: 96 is "4,8". Outside 0 to 100 it gives "".
func formatearNota(puntos float64) string {
	if math.IsNaN(puntos) || puntos <= 0 || puntos > 100 {
		return ""
	}
	decimas := int(math.Round(puntos / 2)) // tenths of a star
	return strconv.Itoa(decimas/10) + "," + strconv.Itoa(decimas%10)
}

// inicial is the first letter of a name, in capitals, for a missing avatar.
func inicial(nombre string) string {
	for _, letra := range nombre {
		return strings.ToUpper(string(letra))
	}
	return ""
}
