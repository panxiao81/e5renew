package view

import (
	"context"
	"embed"
	"html/template"
	"io"

	"github.com/panxiao81/e5renew/internal/i18n"
)

//go:embed *.html
var tmplFS embed.FS

type Template struct {
}

func New() (*Template, error) {
	return &Template{}, nil
}

func (t *Template) Render(w io.Writer, name string, data interface{}) error {
	// Create function map with placeholder i18n functions
	funcMap := template.FuncMap{
		"safeHTML": func(html string) template.HTML {
			return template.HTML(html)
		},
		"dict": func(values ...interface{}) map[string]interface{} {
			if len(values)%2 != 0 {
				return nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil
				}
				dict[key] = values[i+1]
			}
			return dict
		},
		"t": func(messageID string, templateData ...map[string]interface{}) string {
			return messageID // Return message ID as fallback
		},
		"tDefault": func(messageID, defaultMessage string, templateData ...map[string]interface{}) string {
			return defaultMessage // Return default message as fallback
		},
	}

	// Parse templates individually to avoid block name conflicts
	tmpl := template.New("").Funcs(funcMap)
	tmpl, err := tmpl.ParseFS(tmplFS, "layout.html", name)
	if err != nil {
		return err
	}

	return tmpl.ExecuteTemplate(w, name, data)
}

func (t *Template) RenderWithContext(ctx context.Context, w io.Writer, name string, data interface{}) error {
	localizer := i18n.FromContext(ctx)

	// Create enhanced function map with i18n support
	funcMap := template.FuncMap{
		"safeHTML": func(html string) template.HTML {
			return template.HTML(html)
		},
		"dict": func(values ...interface{}) map[string]interface{} {
			if len(values)%2 != 0 {
				return nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil
				}
				dict[key] = values[i+1]
			}
			return dict
		},
		"t": func(messageID string, templateData ...map[string]interface{}) string {
			var data map[string]interface{}
			if len(templateData) > 0 {
				data = templateData[0]
			}
			return localizer.T(messageID, data)
		},
		"tDefault": func(messageID, defaultMessage string, templateData ...map[string]interface{}) string {
			var data map[string]interface{}
			if len(templateData) > 0 {
				data = templateData[0]
			}
			return localizer.TDefault(messageID, defaultMessage, data)
		},
	}

	// Parse templates individually to avoid block name conflicts
	tmpl := template.New("").Funcs(funcMap)
	tmpl, err := tmpl.ParseFS(tmplFS, "layout.html", name)
	if err != nil {
		return err
	}

	return tmpl.ExecuteTemplate(w, name, data)
}
