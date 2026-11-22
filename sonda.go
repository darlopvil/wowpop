package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// sondear sends one real request to Wallapop (or the two or three a page
// needs), prints a summary to salida and returns the exit code. It is the
// way to check from the server that Go gets the same answers as curl, and
// with crudo it dumps the JSON to volcado to map fields from real responses.
//
//	busqueda=<keywords>   one search, at the default location
//	ficha=<item slug>     the listing page, then its data
//	vendedor=<user slug>  the profile page, its data, then its items
func sondear(orden string, crudo bool, volcado, salida io.Writer) int {
	tipo, valor, ok := strings.Cut(orden, "=")
	if !ok || strings.TrimSpace(valor) == "" {
		fmt.Fprintln(salida, "usage: -sonda busqueda=<keywords> | ficha=<item slug> | vendedor=<user slug>")
		return 2
	}
	ctx, cancelar := context.WithTimeout(context.Background(), time.Minute)
	defer cancelar()

	s := &sonda{ctx: ctx, crudo: crudo, volcado: volcado, salida: salida}
	switch tipo {
	case "busqueda":
		s.busqueda(valor)
	case "ficha":
		s.pagina("item", valor)
	case "vendedor":
		s.pagina("user", valor)
	default:
		fmt.Fprintf(salida, "unknown probe %q\n", tipo)
		return 2
	}
	if s.fallo {
		return 1
	}
	return 0
}

type sonda struct {
	ctx     context.Context
	crudo   bool
	volcado io.Writer
	salida  io.Writer
	fallo   bool
}

// pedir sends one request and prints its status line. It returns the
// decoded JSON, or nil when the request failed or the body is not JSON.
func (s *sonda) pedir(etiqueta, direccion string, html bool) (respuesta, map[string]any) {
	var r respuesta
	var err error
	if html {
		r, err = wallapop.pedirHTML(s.ctx, direccion)
	} else {
		r, err = wallapop.pedirJSON(s.ctx, direccion)
	}
	if err != nil {
		fmt.Fprintf(s.salida, "%-15s error: %v\n", etiqueta, err)
		s.fallo = true
		return r, nil
	}
	fmt.Fprintf(s.salida, "%-15s %d  %d B  %d ms\n", etiqueta, r.Estado, len(r.Cuerpo), r.Duracion.Milliseconds())
	if r.Estado < 200 || r.Estado > 299 {
		s.fallo = true
	}
	if html {
		return r, nil
	}
	if s.crudo {
		_, _ = s.volcado.Write(r.Cuerpo)
		_, _ = io.WriteString(s.volcado, "\n")
	}
	var datos map[string]any
	if err := json.Unmarshal(r.Cuerpo, &datos); err != nil {
		fmt.Fprintf(s.salida, "%-15s not JSON: %v\n", "", err)
		s.fallo = true
		return r, nil
	}
	return r, datos
}

func (s *sonda) linea(clave string, valor any) {
	fmt.Fprintf(s.salida, "%-15s %s: %s\n", "", clave, texto(valor))
}

func (s *sonda) busqueda(palabras string) {
	_, d := s.pedir("search", wallapop.urlBusqueda(palabras, latitudPorDefecto, longitudPorDefecto), false)
	if d == nil {
		return
	}
	articulos, _ := campo(d, "data", "section", "items").([]any)
	s.linea("items", len(articulos))
	if len(articulos) > 0 {
		s.linea("first", campo(articulos[0], "title"))
	}
	s.linea("next_page", campo(d, "meta", "next_page") != nil)
}

// pagina probes a listing ("item") or a profile ("user") the way wowpop
// will load it: the HTML page gives the buildId, then the data.
func (s *sonda) pagina(tipo, slug string) {
	r, _ := s.pedir(tipo+" page", wallapop.urlPagina(tipo, slug), true)
	if s.fallo {
		return
	}
	buildID, err := leerBuildID(r.Cuerpo)
	if err != nil {
		fmt.Fprintf(s.salida, "%-15s %v\n", "", err)
		s.fallo = true
		return
	}
	s.linea("buildId", buildID)

	_, d := s.pedir(tipo+" data", wallapop.urlDatos(buildID, tipo, slug), false)
	if d == nil {
		return
	}
	props := campo(d, "pageProps")
	if tipo == "item" {
		s.linea("id", campo(props, "item", "id"))
		s.linea("title", campo(props, "item", "title"))
		s.linea("seller", campo(props, "itemSeller", "microName"))
		return
	}

	id, _ := campo(props, "user", "id").(string)
	s.linea("id", id)
	s.linea("alias", campo(props, "user", "microName"))
	publicados, _ := campo(props, "publishedItems", "data").([]any)
	s.linea("published", len(publicados))
	s.linea("published.meta", campo(props, "publishedItems", "meta"))
	if id == "" {
		return
	}

	_, d = s.pedir("user items", wallapop.urlArticulosVendedor(id, ""), false)
	if d == nil {
		return
	}
	articulos, _ := campo(d, "data").([]any)
	s.linea("items", len(articulos))
	s.linea("meta.next", campo(d, "meta", "next") != nil)
}

// campo walks a decoded JSON value down the given keys, nil when a key is
// missing.
func campo(v any, claves ...string) any {
	for _, c := range claves {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[c]
	}
	return v
}

// texto shows any JSON value on one line, cut at 100 characters: strings as
// they are, everything else as compact JSON, so an unexpected shape is
// visible instead of hidden.
func texto(v any) string {
	var t string
	switch x := v.(type) {
	case string:
		t = x
	case nil:
		t = "(missing)"
	default:
		b, _ := json.Marshal(x)
		t = string(b)
	}
	if r := []rune(t); len(r) > 100 {
		t = string(r[:100]) + "…"
	}
	return t
}
