package controller

import (
	"bytes"
	"fmt"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
)

// registerSprigFunctions to get a new template with sprig functions support
func registerSprigFunctions(tmplStr string, functions *TemplateFuncs) (*template.Template, error) {
	// Create a new template and register Sprig functions
	tmpl, err := template.New("template").
		Funcs(sprig.TxtFuncMap()).
		Funcs(functions.funcMap).
		Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %w", err)
	}
	return tmpl, err
}

// renderTemplate using template vars
func renderTemplate(tmplStr string, vars *TemplateVars, functions *TemplateFuncs) (string, error) {
	tmpl, err := registerSprigFunctions(tmplStr, functions)
	if err != nil {
		return "", err
	}

	// Execute the template with the provided struct
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}
