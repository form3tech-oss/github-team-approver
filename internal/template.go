package internal

import (
	"bytes"
	"github.com/google/go-github/v28/github"
	"text/template"
)

func Render(e github.PullRequestEvent, t string) (string, error){
	tmpl, err := template.New("test").Parse(t)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	err = tmpl.Execute(&tpl, e)
	if err != nil {
		return "", err
	}

	return tpl.String(), nil
}

