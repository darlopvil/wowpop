package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Wallapop only knows "the page after this one": each page of results comes
// with a token for the next. To offer numbered pages, and to go back to
// them, wowpop keeps the tokens of each search in memory, under a random key
// that goes in the page links (&b=…&p=3). Only the tokens and what the
// search was are kept, never any result, and only for as long as Wallapop
// honours the tokens. Everything is lost on restart, and an unknown key
// just means the search has to be done again.

const (
	// maxBusquedas and maxPaginas bound the memory: at about 600 bytes a
	// token, the worst case is a few MB.
	maxBusquedas = 500
	maxPaginas   = 25
	// Wallapop's tokens last a day; wowpop drops them a little earlier.
	vidaBusqueda = 23 * time.Hour
)

type busquedaGuardada struct {
	palabras, orden, id string
	// tokens[n] opens page n+1; tokens[0] is empty, page 1 being the
	// search itself.
	tokens []string
	usada  time.Time
}

type almacenBusquedas struct {
	mu    sync.Mutex
	m     map[string]*busquedaGuardada
	ahora func() time.Time
}

var busquedas = nuevoAlmacen()

func nuevoAlmacen() *almacenBusquedas {
	return &almacenBusquedas{m: map[string]*busquedaGuardada{}, ahora: time.Now}
}

// crear remembers a new search and returns its key. When full, it forgets
// the expired searches and then the least recently used one.
func (a *almacenBusquedas) crear(palabras, orden, id string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	clave := hex.EncodeToString(b[:])

	a.mu.Lock()
	defer a.mu.Unlock()
	ahora := a.ahora()
	if len(a.m) >= maxBusquedas {
		var vieja string
		for k, v := range a.m {
			if ahora.Sub(v.usada) > vidaBusqueda {
				delete(a.m, k)
			} else if vieja == "" || v.usada.Before(a.m[vieja].usada) {
				vieja = k
			}
		}
		if len(a.m) >= maxBusquedas {
			delete(a.m, vieja)
		}
	}
	a.m[clave] = &busquedaGuardada{palabras: palabras, orden: orden, id: id, tokens: []string{""}, usada: ahora}
	return clave
}

// pagina returns what is needed to open a page of a remembered search: its
// token (empty for page 1), the search_id and how many pages are known. ok
// is false if the key is unknown or expired, belongs to another search, or
// the page is not known yet.
func (a *almacenBusquedas) pagina(clave, palabras, orden string, n int) (token, id string, conocidas int, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.m[clave]
	ahora := a.ahora()
	if b == nil || ahora.Sub(b.usada) > vidaBusqueda {
		delete(a.m, clave)
		return "", "", 0, false
	}
	if b.palabras != palabras || b.orden != orden || n < 1 || n > len(b.tokens) {
		return "", "", 0, false
	}
	b.usada = ahora
	return b.tokens[n-1], b.id, len(b.tokens), true
}

// anotar records the token of the page after page n, if that page was the
// last one known, and returns how many pages are known. Pages already known
// are kept as they are, so going back never loses the ones after.
func (a *almacenBusquedas) anotar(clave string, n int, siguiente string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.m[clave]
	if b == nil {
		return 0
	}
	if n == len(b.tokens) && n < maxPaginas {
		b.tokens = append(b.tokens, siguiente)
	}
	return len(b.tokens)
}

// conocidas is how many pages of a search are known.
func (a *almacenBusquedas) conocidas(clave string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if b := a.m[clave]; b != nil {
		return len(b.tokens)
	}
	return 0
}
