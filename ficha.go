package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// /item/<slug> is a listing, at the same path as on es.wallapop.com, so a
// redirect rule only has to change the host.

// reSlug is the shape of the website's slugs: words and the listing number.
var reSlug = regexp.MustCompile(`^[A-Za-z0-9-]{1,200}$`)

// What wowpop reads from a listing's data. Like in searches, what it does
// not show (the seller's coordinates and postal code, ids, counters it
// does not use) is not decoded.
type datosFicha struct {
	PageProps struct {
		Item       fichaWallapop    `json:"item"`
		ItemSeller vendedorWallapop `json:"itemSeller"`
	} `json:"pageProps"`
}

type textoOriginal struct {
	Original string `json:"original"`
}

type fichaWallapop struct {
	Title           textoOriginal `json:"title"`
	Description     textoOriginal `json:"description"`
	Characteristics string        `json:"characteristics"`
	Taxonomies      []struct {
		Name string `json:"name"`
	} `json:"taxonomies"`
	Slug         string `json:"slug"`
	ModifiedDate int64  `json:"modifiedDate"`
	Images       []struct {
		URLs struct {
			Small string `json:"small"`
			Big   string `json:"big"`
		} `json:"urls"`
	} `json:"images"`
	Price struct {
		Cash struct {
			Amount   float64 `json:"amount"`
			Currency string  `json:"currency"`
		} `json:"cash"`
	} `json:"price"`
	Location struct {
		City string `json:"city"`
	} `json:"location"`
	Flags struct {
		Reserved bool `json:"reserved"`
		Bumped   bool `json:"bumped"`
		Sold     bool `json:"sold"`
		Expired  bool `json:"expired"`
	} `json:"flags"`
	Shipping struct {
		IsItemShippable         bool `json:"isItemShippable"`
		IsShippingAllowedByUser bool `json:"isShippingAllowedByUser"`
	} `json:"shipping"`
	Views     int `json:"views"`
	Favorites int `json:"favorites"`
	Condition *struct {
		Title string `json:"title"`
		Text  string `json:"text"`
	} `json:"condition"`
	CharacteristicsDetails []struct {
		Attribute string `json:"attribute"`
		Value     string `json:"value"`
	} `json:"characteristicsDetails"`
	IsRefurbished bool `json:"isRefurbished"`
}

type vendedorWallapop struct {
	MicroName   string `json:"microName"`
	WebSlug     string `json:"webSlug"`
	AvatarImage string `json:"avatarImage"`
	SellerType  string `json:"sellerType"`
	Location    struct {
		City string `json:"city"`
	} `json:"location"`
	Stats statsWallapop `json:"stats"`
}

// ficha is what the listing template shows.
type ficha struct {
	Titulo, Precio, Descripcion, Categorias string
	Fotos                                   []fotoFicha
	Detalles                                []detalle
	Ciudad, Fecha                           string
	Visitas, Favoritos                      int
	Vendido, Caducado, Reservado            bool
	Envio, Destacado, Reacondicionado       bool
	// EnWallapop is the listing on the website, the one way to buy it or
	// write to the seller. It is a plain link: nothing loads from it.
	EnWallapop string
	Vendedor   vendedorFicha
}

// fotoFicha is one photo of the gallery: Anterior and Siguiente are the
// numbers the arrows go to, wrapping around; Completa is the biggest size,
// opened in its own tab.
type fotoFicha struct {
	Numero, Anterior, Siguiente, Total int
	Grande, Chica, Completa            string
}

// anchoCompleto is the biggest width the image server really has: asked
// for more, it answers with a small placeholder instead.
const anchoCompleto = "W1024"

type detalle struct{ Nombre, Valor string }

type vendedorFicha struct {
	Alias, Enlace, Avatar, Inicial, Ciudad, Nota string
	Profesional                                  bool
	Valoraciones, Ventas                         int
}

func paginaFicha(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/item/")
	if !reSlug.MatchString(slug) {
		fichaNoEncontrada(w)
		return
	}

	ctx, cancelar := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancelar()
	res, err := wallapop.pedirDatosWeb(ctx, "item", slug)
	var datos datosFicha
	if err == nil && res.Estado == http.StatusOK {
		err = json.Unmarshal(res.Cuerpo, &datos)
	}
	switch {
	case err != nil:
		log.Printf("listing: %v", err)
		errorFicha(w)
		return
	case res.Estado == http.StatusNotFound || res.Estado == http.StatusGone:
		fichaNoEncontrada(w)
		return
	case res.Estado != http.StatusOK:
		log.Printf("listing: Wallapop answered %d", res.Estado)
		errorFicha(w)
		return
	}
	f := nuevaFicha(datos.PageProps.Item, datos.PageProps.ItemSeller, slug)
	if f.Titulo == "" {
		// A 200 without a listing in it: treat it as gone.
		fichaNoEncontrada(w)
		return
	}
	renderizar(w, http.StatusOK, "ficha", datosPagina{Titulo: f.Titulo, Ficha: f})
}

func fichaNoEncontrada(w http.ResponseWriter) {
	renderizar(w, http.StatusNotFound, "error", datosPagina{
		Titulo:  "Anuncio no encontrado",
		Mensaje: "Este anuncio no existe o ya se ha retirado.",
	})
}

func errorFicha(w http.ResponseWriter) {
	renderizar(w, http.StatusBadGateway, "error", datosPagina{
		Titulo:  "Wallapop no responde",
		Mensaje: "No se ha podido abrir el anuncio ahora mismo. Prueba de nuevo en un rato.",
	})
}

func nuevaFicha(a fichaWallapop, v vendedorWallapop, slug string) *ficha {
	f := &ficha{
		Titulo:          strings.TrimSpace(a.Title.Original),
		Precio:          formatearPrecio(a.Price.Cash.Amount, a.Price.Cash.Currency),
		Descripcion:     strings.TrimSpace(a.Description.Original),
		Ciudad:          a.Location.City,
		Fecha:           formatearFecha(a.ModifiedDate),
		Visitas:         a.Views,
		Favoritos:       a.Favorites,
		Vendido:         a.Flags.Sold,
		Caducado:        a.Flags.Expired,
		Reservado:       a.Flags.Reserved,
		Envio:           a.Shipping.IsItemShippable && a.Shipping.IsShippingAllowedByUser,
		Destacado:       a.Flags.Bumped,
		Reacondicionado: a.IsRefurbished,
		EnWallapop:      "https://es.wallapop.com/item/" + url.PathEscape(slug),
	}

	var categorias []string
	for _, t := range a.Taxonomies {
		if t.Name != "" {
			categorias = append(categorias, t.Name)
		}
	}
	f.Categorias = strings.Join(categorias, " › ")

	for _, img := range a.Images {
		grande, chica := rutaFoto(img.URLs.Big, "W800"), rutaFoto(img.URLs.Small, "W320")
		if grande != "" && chica != "" {
			f.Fotos = append(f.Fotos, fotoFicha{
				Numero:   len(f.Fotos) + 1,
				Grande:   grande,
				Chica:    chica,
				Completa: rutaFoto(img.URLs.Big, anchoCompleto),
			})
		}
	}
	for i := range f.Fotos {
		n := len(f.Fotos)
		f.Fotos[i].Total = n
		f.Fotos[i].Anterior = (i+n-1)%n + 1
		f.Fotos[i].Siguiente = (i+1)%n + 1
	}

	if c := a.Condition; c != nil && c.Text != "" {
		f.Detalles = append(f.Detalles, detalle{Nombre: valorO(c.Title, "Estado"), Valor: c.Text})
	}
	for _, d := range a.CharacteristicsDetails {
		if d.Attribute != "" && d.Value != "" {
			f.Detalles = append(f.Detalles, detalle{Nombre: d.Attribute, Valor: d.Value})
		}
	}
	if len(f.Detalles) == 0 && a.Characteristics != "" {
		f.Detalles = append(f.Detalles, detalle{Nombre: "Características", Valor: a.Characteristics})
	}

	f.Vendedor = vendedorFicha{
		Alias:        v.MicroName,
		Avatar:       rutaFoto(v.AvatarImage, "W320"),
		Ciudad:       v.Location.City,
		Profesional:  strings.EqualFold(v.SellerType, "pro"),
		Valoraciones: v.Stats.Counters.Reviews,
		Ventas:       v.Stats.Counters.Sells,
	}
	if f.Vendedor.Valoraciones > 0 {
		f.Vendedor.Nota = formatearNota(v.Stats.Ratings.Reviews)
	}
	if reSlug.MatchString(v.WebSlug) {
		f.Vendedor.Enlace = "/user/" + v.WebSlug
	}
	if f.Vendedor.Avatar == "" {
		f.Vendedor.Inicial = inicial(v.MicroName)
	}
	return f
}

func valorO(v, porDefecto string) string {
	if v == "" {
		return porDefecto
	}
	return v
}

// madrid is the time zone dates are shown in; the binary carries its own
// copy of the zone database (time/tzdata).
var madrid = func() *time.Location {
	l, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		panic(err) // embedded tzdata: only a broken build gets here
	}
	return l
}()

// formatearFecha writes a Wallapop date (milliseconds since 1970) as
// 22/11/2026, or "" when there is none.
func formatearFecha(ms int64) string {
	if ms <= 0 {
		return ""
	}
	t := time.UnixMilli(ms).In(madrid)
	return strconv.Itoa(t.Day()) + "/" + strconv.Itoa(int(t.Month())) + "/" + strconv.Itoa(t.Year())
}
