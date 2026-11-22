package main

import (
	"fmt"
	"testing"
	"time"
)

// almacenDePrueba is a store whose clock the test moves.
func almacenDePrueba() (*almacenBusquedas, *time.Time) {
	a := nuevoAlmacen()
	ahora := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	a.ahora = func() time.Time { return ahora }
	return a, &ahora
}

func TestARememberedSearchGivesItsPages(t *testing.T) {
	a, _ := almacenDePrueba()
	clave := a.crear("game boy", "baratos", "id-1")
	if tok, id, n, ok := a.pagina(clave, "game boy", "baratos", 1); !ok || tok != "" || id != "id-1" || n != 1 {
		t.Fatalf("page 1: %q %q %d %v", tok, id, n, ok)
	}
	if a.anotar(clave, 1, "t2") != 2 || a.anotar(clave, 2, "t3") != 3 {
		t.Fatal("pages not recorded")
	}
	if tok, _, n, ok := a.pagina(clave, "game boy", "baratos", 3); !ok || tok != "t3" || n != 3 {
		t.Errorf("page 3: %q %d %v", tok, n, ok)
	}
	// Going back to page 1 and getting a new token does not change page 2.
	if a.anotar(clave, 1, "otro") != 3 {
		t.Error("known pages must stay")
	}
	if tok, _, _, _ := a.pagina(clave, "game boy", "baratos", 2); tok != "t2" {
		t.Errorf("page 2 changed to %q", tok)
	}
}

func TestPagesNotReachedOrOfAnotherSearchAreRefused(t *testing.T) {
	a, _ := almacenDePrueba()
	clave := a.crear("x", "", "id")
	for _, c := range []struct {
		palabras, orden string
		n               int
	}{{"x", "", 2}, {"x", "", 0}, {"y", "", 1}, {"x", "caros", 1}} {
		if _, _, _, ok := a.pagina(clave, c.palabras, c.orden, c.n); ok {
			t.Errorf("%+v should be refused", c)
		}
	}
	if _, _, _, ok := a.pagina("ffffffffffffffff", "x", "", 1); ok {
		t.Error("unknown key")
	}
}

func TestSearchesExpireBeforeWallapopsTokens(t *testing.T) {
	a, ahora := almacenDePrueba()
	clave := a.crear("x", "", "id")
	*ahora = ahora.Add(vidaBusqueda + time.Minute)
	if _, _, _, ok := a.pagina(clave, "x", "", 1); ok {
		t.Fatal("an expired search must be forgotten")
	}
	if a.conocidas(clave) != 0 {
		t.Error("and removed")
	}
}

func TestTheStoreIsBounded(t *testing.T) {
	a, ahora := almacenDePrueba()
	primera := a.crear("primera", "", "id")
	for i := 0; i < maxBusquedas+50; i++ {
		*ahora = ahora.Add(time.Second)
		a.crear(fmt.Sprint(i), "", "id")
	}
	if len(a.m) > maxBusquedas {
		t.Errorf("%d searches kept, at most %d", len(a.m), maxBusquedas)
	}
	if a.conocidas(primera) != 0 {
		t.Error("the least recently used search goes first")
	}

	clave := a.crear("muchas", "", "id")
	for n := 1; n <= maxPaginas+10; n++ {
		a.anotar(clave, n, "t")
	}
	if a.conocidas(clave) != maxPaginas {
		t.Errorf("%d pages kept, at most %d", a.conocidas(clave), maxPaginas)
	}
}

func TestKeysAreRandomAndWellFormed(t *testing.T) {
	a, _ := almacenDePrueba()
	vistas := map[string]bool{}
	for i := 0; i < 100; i++ {
		c := a.crear("x", "", "id")
		if !reClave.MatchString(c) || vistas[c] {
			t.Fatalf("bad or repeated key %q", c)
		}
		vistas[c] = true
	}
}
