package internal

import (
	"bytes"
	"github.com/google/go-github/v28/github"
	"text/template"
)

func Render(e github.PullRequestEvent, t string) ([]byte, error){
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

