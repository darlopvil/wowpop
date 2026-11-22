package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"time"
)

// Static files are embedded in the binary, so they only change with a new
// build. Each one gets a version, a hash of its content, and pages link to
// it as /static/estilo.css?v=<version>. With the right version the browser
// keeps the file for a year without asking again, and a new build changes
// the link. Without it (an old page, /favicon.ico, the licence links) the
// file is still served, but the browser has to check first with its ETag,
// so it never keeps a stale copy.

type archivoEstatico struct {
	contenido []byte
	version   string
}

// Types Go does not know without a system mime table, which the scratch
// image does not have.
var tiposEstaticos = map[string]string{".woff2": "font/woff2"}

var estaticos = cargarEstaticos()

func cargarEstaticos() map[string]archivoEstatico {
	m := map[string]archivoEstatico{}
	err := fs.WalkDir(estaticosFS, "static", func(ruta string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := estaticosFS.ReadFile(ruta)
		if err != nil {
			return err
		}
		m["/"+ruta] = archivoEstatico{contenido: b, version: versionDe(b)}
		return nil
	})
	if err != nil {
		panic(err) // embedded files: only a broken build gets here
	}
	// The stylesheet loads the fonts: point it to their versioned URLs and
	// version the result, so a new font also changes the stylesheet's link.
	const hoja = "/static/estilo.css"
	css := m[hoja]
	for ruta, a := range m {
		css.contenido = bytes.ReplaceAll(css.contenido, []byte(`url("`+ruta+`")`), []byte(`url("`+ruta+"?v="+a.version+`")`))
	}
	css.version = versionDe(css.contenido)
	m[hoja] = css
	return m
}

func versionDe(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:6])
}

// estatico is the template function that links a static file with its
// version: {{ estatico "estilo.css" }}.
func estatico(nombre string) string {
	ruta := "/static/" + nombre
	if a, ok := estaticos[ruta]; ok {
		return ruta + "?v=" + a.version
	}
	return ruta
}

func paginaEstatico(w http.ResponseWriter, r *http.Request) {
	a, ok := estaticos[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	c := w.Header()
	if r.URL.Query().Get("v") == a.version {
		c.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Set("Cache-Control", "no-cache")
	}
	c.Set("ETag", `"`+a.version+`"`)
	if t, ok := tiposEstaticos[path.Ext(r.URL.Path)]; ok {
		c.Set("Content-Type", t)
	}
	// ServeContent answers If-None-Match with a 304 and sets the content
	// type of everything else from the extension.
	http.ServeContent(w, r, path.Base(r.URL.Path), time.Time{}, bytes.NewReader(a.contenido))
}
