package controller

import (
	"bytes"
	"fmt"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
)

func renderTemplate(tmplStr string, vars *TemplateVars) (string, error) {
	// Create a new template and register Sprig functions
	tmpl, err := template.New("template").
		Funcs(sprig.TxtFuncMap()).
		Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("error parsing template: %w", err)
	}

	// Execute the template with the provided struct
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}
