package main

import (
	"context"
	"net/http"
	"testing"
)

func TestDataRequestsReuseTheBuildUntilItFails(t *testing.T) {
	web := &webDePrueba{build: "b1", datos: `{"ok":1}`}
	wallapopDePrueba(t, nil, web.ServeHTTP)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		res, err := wallapop.pedirDatosWeb(ctx, "item", "iphone-13-blanco-nuevo-1326730384")
		if err != nil || res.Estado != http.StatusOK || string(res.Cuerpo) != `{"ok":1}` {
			t.Fatalf("request %d: %v %d %q", i, err, res.Estado, res.Cuerpo)
		}
	}
	if web.paginas != 1 {
		t.Errorf("the page was read %d times, once is enough", web.paginas)
	}
}

func TestAPageThatDoesNotExistIsAnswered404(t *testing.T) {
	web := &webDePrueba{build: "b1", estadoPagina: http.StatusNotFound}
	wallapopDePrueba(t, nil, web.ServeHTTP)
	res, err := wallapop.pedirDatosWeb(context.Background(), "item", "nada-1")
	if err != nil || res.Estado != http.StatusNotFound || web.pedidos != 0 {
		t.Fatalf("%v %d, %d data requests", err, res.Estado, web.pedidos)
	}
}

func TestAPageWithoutBuildIsAnError(t *testing.T) {
	wallapopDePrueba(t, nil, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>no build here</html>"))
	})
	if _, err := wallapop.pedirDatosWeb(context.Background(), "item", "x-1"); err == nil {
		t.Fatal("expected an error")
	}
}
