package export

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
)

func TestColumnName(t *testing.T) {
	for i, want := range map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 701: "ZZ", 702: "AAA"} {
		if got := ColumnName(i); got != want {
			t.Errorf("ColumnName(%d) = %s, se esperaba %s", i, got, want)
		}
	}
}

func TestWrite_ProducesAValidWorkbook(t *testing.T) {
	var buf bytes.Buffer
	err := NewXLSXWriter().Write(&buf, []ports.Sheet{
		{Name: "Alertas", Rows: [][]string{{"ID de alerta", "Etiqueta"}, {"ALR-000123", "Otro: <prueba> & \"x\"\x01"}}},
		{Name: "Parámetros", Rows: [][]string{{"Filtro", "Valor"}, {"Dispositivo", "Todos"}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("no es un zip: %v", err)
	}
	contents := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		contents[f.Name] = string(b)
		// Todo el XML debe estar bien formado.
		decoder := xml.NewDecoder(bytes.NewReader(b))
		for {
			if _, err := decoder.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Fatalf("%s mal formado: %v", f.Name, err)
			}
		}
	}
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml"} {
		if _, ok := contents[name]; !ok {
			t.Fatalf("falta %s", name)
		}
	}
	if !strings.Contains(contents["xl/workbook.xml"], `name="Parámetros"`) {
		t.Fatal("falta la hoja Parámetros")
	}
	sheet := contents["xl/worksheets/sheet1.xml"]
	if !strings.Contains(sheet, `<c r="B2" t="inlineStr"><is><t xml:space="preserve">Otro: &lt;prueba&gt; &amp; &#34;x&#34;</t></is></c>`) {
		t.Fatalf("celda B2 mal escrita: %s", sheet)
	}
}
