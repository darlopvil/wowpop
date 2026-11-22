package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
)

// datosPagina is what every template receives. Each new page adds its own
// fields here.
type datosPagina struct {
	Titulo   string
	Mensaje  string
	Consulta string
	// Inicio hides the header search box on the home page, which has its own
	// big one.
	Inicio bool
	// Resultados is a page of search results.
	Resultados *resultadosBusqueda
	// Volver, on an error page, links a fresh copy of what failed instead
	// of the home page.
	Volver string
}

var funciones = template.FuncMap{
	"estatico": estatico,
}

// Each page is parsed into its own set together with the base template,
// because every page defines the same "contenido" block.
func cargarPlantilla(pagina string) *template.Template {
	archivos := []string{"templates/base.html", "templates/" + pagina + ".html"}
	return template.Must(template.New("base.html").Funcs(funciones).ParseFS(plantillasFS, archivos...))
}

var plantillas = map[string]*template.Template{
	"inicio":   cargarPlantilla("inicio"),
	"about":    cargarPlantilla("about"),
	"error":    cargarPlantilla("error"),
	"busqueda": cargarPlantilla("busqueda"),
}

// renderizar executes the template into a buffer before writing anything, so
// a failure halfway through the page becomes a clean 500 instead of half a
// page served with a 200.
func renderizar(w http.ResponseWriter, estado int, pagina string, datos datosPagina) {
	var buf bytes.Buffer
	if err := plantillas[pagina].ExecuteTemplate(&buf, "base.html", datos); err != nil {
		log.Printf("error rendering %s: %v", pagina, err)
		http.Error(w, "Error interno", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(estado)
	_, _ = buf.WriteTo(w)
}
