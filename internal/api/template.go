package api

import (
	"bytes"
	"text/template"
)

func renderTemplate(e event, t string) ([]byte, error) {
	tmpl, err := template.New("test").Parse(t)
	if err != nil {
		return nil, err
	}

	var tpl bytes.Buffer
	err = tmpl.Execute(&tpl, e)
	if err != nil {
		return nil, err
	}

	return tpl.Bytes(), nil
}
