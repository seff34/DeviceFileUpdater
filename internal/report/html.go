package report

import (
	_ "embed"
	"html/template"
	"io"

	"devupdater/internal/model"
)

//go:embed report.html.tmpl
var htmlTmpl string

var tmpl = template.Must(template.New("report").Parse(htmlTmpl))

var statusOrder = []model.Status{model.Created, model.Updated, model.Unchanged, model.WouldCreate, model.WouldUpdate, model.Failed}

func WriteHTML(w io.Writer, r model.RunResult) error {
	return tmpl.Execute(w, map[string]any{
		"Run":         r,
		"Counts":      Summarize(r),
		"StatusOrder": statusOrder,
		"Duration":    r.Finished.Sub(r.Started).Round(1e9).String(),
	})
}
