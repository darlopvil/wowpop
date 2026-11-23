package main

import (
	"context"
	"net/http"
)

// Listings and profiles come from the website's own Next.js data routes,
// /_next/data/<build>/es/<item|user>/<slug>.json: the JSON a browser loads
// when it moves between pages of es.wallapop.com. The build changes with
// every release of the website, so wowpop reads it once from an ordinary
// page and keeps it until a data URL answers 404; then it reads the page of
// what was asked for again, which also tells whether that still exists.

// pedirDatosWeb returns the data of a listing ("item") or a profile
// ("user"). A 404 means the page does not exist on the website either.
func (c *clienteWallapop) pedirDatosWeb(ctx context.Context, tipo, slug string) (respuesta, error) {
	c.mu.Lock()
	build := c.buildID
	c.mu.Unlock()

	if build != "" {
		res, err := c.pedirJSON(ctx, c.urlDatos(build, tipo, slug))
		if err != nil || res.Estado != http.StatusNotFound {
			return res, err
		}
	}

	// No build yet, or it has changed: read the page itself.
	pagina, err := c.pedirHTML(ctx, c.urlPagina(tipo, slug))
	if err != nil {
		return respuesta{}, err
	}
	if pagina.Estado != http.StatusOK {
		return pagina, nil
	}
	nuevo, err := leerBuildID(pagina.Cuerpo)
	if err != nil {
		return respuesta{}, err
	}
	c.mu.Lock()
	c.buildID = nuevo
	c.mu.Unlock()
	return c.pedirJSON(ctx, c.urlDatos(nuevo, tipo, slug))
}
