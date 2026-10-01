package workspace

import (
	"bytes"
	"encoding/csv"
	"io"
	"strings"
)

// The workspace CSV files are written the way Excel in a Turkish locale reads
// and saves them: ';' between fields and a UTF-8 BOM, without which Excel puts
// every row in one column and garbles non-ASCII characters.
const (
	csvSep  = ';'
	utf8BOM = "\xef\xbb\xbf"
)

// newCSVReader reads either form: ';' (as written here and by Excel in many
// locales) or ',' (older files and other tools). The header decides: a header
// with ';' and no ',' can only be the ';' form.
func newCSVReader(data []byte) *csv.Reader {
	data = bytes.TrimPrefix(data, []byte(utf8BOM))
	cr := csv.NewReader(bytes.NewReader(data))
	if header, _, _ := strings.Cut(string(data), "\n"); strings.Contains(header, ";") && !strings.Contains(header, ",") {
		cr.Comma = ';'
	}
	return cr
}

// newCSVWriter writes the BOM and returns a ';'-separated writer.
func newCSVWriter(w io.Writer) (*csv.Writer, error) {
	if _, err := io.WriteString(w, utf8BOM); err != nil {
		return nil, err
	}
	cw := csv.NewWriter(w)
	cw.Comma = csvSep
	return cw, nil
}
