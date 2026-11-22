package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// Static files are embedded in the binary, so they only change with a new
// build. Each one gets a version, a hash of its content, and pages link to
// it as /static/estilo.css?v=<version>. With the right version the browser
// keeps the file for a year without asking again, and a new build changes
// the link. Without it (an old page, /favicon.ico, the licence links) the
// file is still served, but the browser has to check first with its ETag,
// so it never keeps a stale copy.
//
// Text files (styles, drawings, licences) are also kept gzipped, compressed
// once at start-up, and sent that way to browsers that accept it.

type archivoEstatico struct {
	contenido []byte
	// gz is the gzipped content, nil when compressing does not pay off.
	gz      []byte
	version string
}

// Types Go does not know without a system mime table, which the scratch
// image does not have.
var tiposEstaticos = map[string]string{
	".woff2": "font/woff2",
	".txt":   "text/plain; charset=utf-8",
}

// comprimibles are the extensions worth gzipping: woff2 is already
// compressed.
var comprimibles = map[string]bool{".css": true, ".svg": true, ".txt": true}

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

	for ruta, a := range m {
		if comprimibles[path.Ext(ruta)] {
			a.gz = comprimir(a.contenido)
			m[ruta] = a
		}
	}
	return m
}

// comprimir gzips b, or returns nil when that does not make it smaller.
func comprimir(b []byte) []byte {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		panic(err) // a fixed, valid level: only a broken build gets here
	}
	_, _ = w.Write(b)
	_ = w.Close()
	if buf.Len() >= len(b) {
		return nil
	}
	return buf.Bytes()
}

// aceptaGzip tells whether the request's Accept-Encoding allows gzip.
func aceptaGzip(r *http.Request) bool {
	for _, parte := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		nombre, parametros, _ := strings.Cut(strings.TrimSpace(parte), ";")
		if strings.TrimSpace(nombre) != "gzip" {
			continue
		}
		return strings.ReplaceAll(strings.TrimSpace(parametros), " ", "") != "q=0"
	}
	return false
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
	// Set the type here: ServeContent would otherwise sniff it from the
	// body, which may be gzipped.
	ext := path.Ext(r.URL.Path)
	tipo, ok := tiposEstaticos[ext]
	if !ok {
		tipo = mime.TypeByExtension(ext)
	}
	if tipo != "" {
		c.Set("Content-Type", tipo)
	}

	cuerpo, etag := a.contenido, a.version
	if a.gz != nil {
		c.Add("Vary", "Accept-Encoding")
		if aceptaGzip(r) {
			cuerpo, etag = a.gz, a.version+"-gz"
			c.Set("Content-Encoding", "gzip")
		}
	}
	c.Set("ETag", `"`+etag+`"`)
	// ServeContent answers If-None-Match with a 304.
	http.ServeContent(w, r, path.Base(r.URL.Path), time.Time{}, bytes.NewReader(cuerpo))
}
