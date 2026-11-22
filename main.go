package main

import (
	"embed"
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	// Embedded time zone database: the scratch image has no
	// /usr/share/zoneinfo, and this keeps the TZ variable working anyway.
	_ "time/tzdata"
)

var (
	//go:embed templates
	plantillasFS embed.FS

	//go:embed static
	estaticosFS embed.FS
)

// entorno returns the value of an environment variable, or the default
// value when it is unset or empty.
func entorno(nombre, porDefecto string) string {
	if v := os.Getenv(nombre); v != "" {
		return v
	}
	return porDefecto
}

func main() {
	// The default host is 0.0.0.0 rather than localhost: inside a container,
	// localhost would leave the process unreachable.
	puertoPorDefecto, err := strconv.Atoi(entorno("WOWPOP_PORT", "8080"))
	if err != nil {
		log.Fatalf("WOWPOP_PORT is not a valid number: %v", err)
	}
	puerto := flag.Int("p", puertoPorDefecto, "Listen port")
	host := flag.String("h", entorno("WOWPOP_HOST", "0.0.0.0"), "Listen address")
	sonda := flag.String("sonda", "", "Send one request to Wallapop and print a summary, then exit: busqueda=<keywords>, ficha=<item slug> or vendedor=<user slug>")
	crudo := flag.Bool("crudo", false, "With -sonda, also write the raw responses to stdout")
	flag.Parse()

	if *sonda != "" {
		os.Exit(sondear(*sonda, *crudo, os.Stdout, os.Stderr))
	}

	srv := &http.Server{
		Addr:              *host + ":" + strconv.Itoa(*puerto),
		Handler:           rutas(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("Listening on %s", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}

// rutas builds every route of the site.
func rutas() http.Handler {
	mux := http.NewServeMux()

	// The home page never talks to Wallapop.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			renderizar(w, http.StatusNotFound, "error", datosPagina{
				Titulo:  "Página no encontrada",
				Mensaje: "Esta dirección no existe.",
			})
			return
		}
		renderizar(w, http.StatusOK, "inicio", datosPagina{Inicio: true})
	})

	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		renderizar(w, http.StatusOK, "about", datosPagina{Titulo: "Acerca de"})
	})

	// Browsers request /favicon.ico without looking at the <head>.
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, estatico("favicon.svg"), http.StatusMovedPermanently)
	})

	mux.HandleFunc("/robots.txt", robots)

	// Versioned static files, see estaticos.go.
	mux.HandleFunc("/static/", paginaEstatico)

	return cabeceras(mux)
}
