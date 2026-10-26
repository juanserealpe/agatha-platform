// Package export escribe el Excel del historial (.xlsx) solo con la stdlib:
// un zip con el XML mínimo de SpreadsheetML (strings inline, encabezado en
// negrita y la primera fila fija). Implementa ports.SpreadsheetWriter.
package export

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/alert_management/application/ports"
)

// XLSXWriter escribe libros .xlsx.
type XLSXWriter struct{}

func NewXLSXWriter() XLSXWriter { return XLSXWriter{} }

const (
	contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
%s</Types>`
	rootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`
	styles = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts>
<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>
<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0" applyFont="1"/></cellXfs>
</styleSheet>`
)

// Write escribe el libro. Los nombres de hoja se recortan a 31 caracteres válidos.
func (XLSXWriter) Write(w io.Writer, sheets []ports.Sheet) error {
	zw := zip.NewWriter(w)

	var overrides, workbookSheets, workbookRels strings.Builder
	for i := range sheets {
		n := i + 1
		fmt.Fprintf(&overrides, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`+"\n", n)
		fmt.Fprintf(&workbookSheets, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, escape(sheetName(sheets[i].Name, n)), n, n)
		fmt.Fprintf(&workbookRels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, n, n)
	}
	stylesID := len(sheets) + 1
	fmt.Fprintf(&workbookRels, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`, stylesID)

	files := []struct{ name, body string }{
		{"[Content_Types].xml", fmt.Sprintf(contentTypes, overrides.String())},
		{"_rels/.rels", rootRels},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` + workbookSheets.String() + `</sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + workbookRels.String() + `</Relationships>`},
		{"xl/styles.xml", styles},
	}
	for _, f := range files {
		if err := writeFile(zw, f.name, f.body); err != nil {
			return err
		}
	}
	for i, s := range sheets {
		if err := writeFile(zw, fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), sheetXML(s)); err != nil {
			return err
		}
	}
	return zw.Close()
}

func writeFile(zw *zip.Writer, name, body string) error {
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(f, body)
	return err
}

func sheetXML(s ports.Sheet) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	if len(s.Rows) > 0 {
		b.WriteString(`<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`)
	}
	b.WriteString(`<sheetData>`)
	for r, row := range s.Rows {
		fmt.Fprintf(&b, `<row r="%d">`, r+1)
		style := ""
		if r == 0 {
			style = ` s="1"`
		}
		for c, value := range row {
			fmt.Fprintf(&b, `<c r="%s%d" t="inlineStr"%s><is><t xml:space="preserve">%s</t></is></c>`, ColumnName(c), r+1, style, escape(value))
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

// ColumnName es la letra de la columna (0 ⇒ A, 25 ⇒ Z, 26 ⇒ AA).
func ColumnName(i int) string {
	name := ""
	for i++; i > 0; i = (i - 1) / 26 {
		name = string(rune('A'+(i-1)%26)) + name
	}
	return name
}

func sheetName(name string, n int) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" {
		return fmt.Sprintf("Hoja%d", n)
	}
	if runes := []rune(name); len(runes) > 31 {
		name = string(runes[:31])
	}
	return name
}

// escape escapa XML y descarta los caracteres de control que XML 1.0 no admite.
func escape(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, s)
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
