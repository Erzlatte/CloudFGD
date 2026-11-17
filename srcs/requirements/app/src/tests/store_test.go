package tests

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"CloudFGD/app/internal/store"
)

func newStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "saved.json")
	st, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	return st, path
}

func TestStoreGuardaYRecarga(t *testing.T) {
	st, path := newStore(t)

	added, err := st.Add(store.SavedItem{Source: "boe", ExternalID: "1", Title: "Ley", Date: "2007-07-12", Tags: []string{"empleo"}})
	if err != nil {
		t.Fatal(err)
	}
	if added.ID == "" || added.SavedAt.IsZero() {
		t.Errorf("faltan id o fecha: %+v", added)
	}

	reloaded, err := store.New(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.List(store.Filter{}); len(got) != 1 || got[0].ID != added.ID {
		t.Errorf("tras recargar = %+v", got)
	}
	if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("no debe quedar el fichero temporal")
	}
}

func TestStoreDuplicado(t *testing.T) {
	st, _ := newStore(t)
	first, _ := st.Add(store.SavedItem{Source: "boe", ExternalID: "1", Title: "Ley"})

	old, err := st.Add(store.SavedItem{Source: "boe", ExternalID: "1", Title: "Otra"})
	if !errors.Is(err, store.ErrDuplicate) || old.ID != first.ID {
		t.Errorf("err = %v, id = %q", err, old.ID)
	}
	if _, err := st.Add(store.SavedItem{Source: "datosgob", ExternalID: "1", Title: "Otra fuente"}); err != nil {
		t.Errorf("mismo id en otra fuente no es duplicado: %v", err)
	}
}

func TestStoreOrdenYFiltros(t *testing.T) {
	st, _ := newStore(t)
	st.Add(store.SavedItem{Source: "boe", ExternalID: "1", Title: "A", Date: "2007-07-12", Tags: []string{"empleo"}})
	st.Add(store.SavedItem{Source: "datosgob", ExternalID: "2", Title: "B", Tags: []string{"paro"}})
	st.Add(store.SavedItem{Source: "boe", ExternalID: "3", Title: "C", Date: "2015-10-02"})

	if got := st.List(store.Filter{}); len(got) != 3 || got[0].Title != "C" || got[2].Title != "A" {
		t.Errorf("orden = %v", titles(got))
	}

	cases := []struct {
		name string
		f    store.Filter
		want int
	}{
		{"fuente", store.Filter{Source: "boe"}, 2},
		{"etiqueta", store.Filter{Tag: "paro"}, 1},
		{"desde", store.Filter{From: "2010-01-01"}, 1},
		{"hasta", store.Filter{To: "2010-01-01"}, 1},
		{"rango sin fecha excluye", store.Filter{From: "1900-01-01"}, 2},
		{"combinado", store.Filter{Source: "boe", Tag: "empleo"}, 1},
	}
	for _, c := range cases {
		if got := st.List(c.f); len(got) != c.want {
			t.Errorf("%s: %d elementos, quiero %d", c.name, len(got), c.want)
		}
	}
}

func TestStoreBorrar(t *testing.T) {
	st, _ := newStore(t)
	it, _ := st.Add(store.SavedItem{Source: "boe", ExternalID: "1", Title: "A"})

	if err := st.Delete(it.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.Delete(it.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("borrar dos veces: %v", err)
	}
	if len(st.List(store.Filter{})) != 0 {
		t.Error("la lista debería estar vacía")
	}
}

func TestStoreFicheroCorrupto(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.json")
	os.WriteFile(path, []byte("{no es json"), 0o644)
	if _, err := store.New(path); err == nil {
		t.Error("debería fallar con un fichero corrupto")
	}
}

func TestStoreWritable(t *testing.T) {
	st, _ := newStore(t)
	if err := st.Writable(); err != nil {
		t.Errorf("el directorio temporal debería ser escribible: %v", err)
	}

	missing, err := store.New(filepath.Join(t.TempDir(), "no-existe", "saved.json"))
	if err != nil {
		t.Fatal(err)
	}
	if missing.Writable() == nil {
		t.Error("un directorio inexistente no es escribible")
	}
}

func titles(items []store.SavedItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}
