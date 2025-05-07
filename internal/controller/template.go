/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"bytes"
	"fmt"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
)

// registerSprigFunctions to get a new template with sprig functions support
func registerSprigFunctions(tmplStr string) (*template.Template, error) {
	// Create a new template and register Sprig functions
	tmpl, err := template.New("template").
		Funcs(sprig.TxtFuncMap()).
		Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %w", err)
	}
	return tmpl, err
}

// renderTemplate using template vars
func renderTemplate(tmplStr string, vars *TemplateVars) (string, error) {
	tmpl, err := registerSprigFunctions(tmplStr)
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
