package server

import (
	"errors"
	"io"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
	"text/template"
	"time"
)

var errTemplateNotFound = errors.New("template not found")

// ExecuteTemplate runs the given template with the value
func (s *Server) ExecuteTemplate(name string, w io.Writer, value interface{}) error {
	if s.templatesPath == "" {
		return s.executeTemplateDynamically(name, w, value)
	}

	t, ok := s.templates[name]
	if !ok {
		return errTemplateNotFound
	}

	return t.Execute(w, value)
}

func (s *Server) loadTemplates() error {
	if s.templatesPath == "" {
		return nil
	}

	baseContent, err := ioutil.ReadFile(filepath.Join(s.templatesPath, "base.html"))
	if err != nil {
		return err
	}

	base, err := template.New("base").Funcs(s.templateFuncs()).Parse(string(baseContent))
	if err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(s.templatesPath)
	if err != nil {
		return err
	}

	s.templates = make(map[string]*template.Template)

	for _, entry := range entries {
		name := entry.Name()
		ext := path.Ext(name)
		if ext != ".html" || name == "base.html" {
			continue
		}
		filename := filepath.Join(s.templatesPath, name)
		content, err := ioutil.ReadFile(filename)
		if err != nil {
			return err
		}

		t, err := parseTemplate(base, string(content))
		if err != nil {
			return err
		}

		s.templates[name[:len(name)-len(ext)]] = t
	}

	return nil
}

func parseTemplate(base *template.Template, content string) (*template.Template, error) {
	t, err := base.Clone()
	if err != nil {
		return nil, err
	}

	_, err = t.New("content").Parse(content)
	return t, err
}

func (s *Server) executeTemplateDynamically(name string, w io.Writer, value interface{}) error {
	_, filename, _, _ := runtime.Caller(1)
	templatePath := path.Join(path.Dir(filename), "./templates")

	baseContent, err := ioutil.ReadFile(filepath.Join(templatePath, "base.html"))
	if err != nil {
		return err
	}

	base, err := template.New("base").Funcs(s.templateFuncs()).Parse(string(baseContent))
	if err != nil {
		return err
	}

	content, err := ioutil.ReadFile(filepath.Join(templatePath, name+".html"))
	if err != nil {
		return err
	}

	t, err := parseTemplate(base, string(content))
	if err != nil {
		return err
	}

	return t.Execute(w, value)
}

func (s *Server) templateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"formatDuration": func(d time.Duration) string {
			return d.Round(time.Second).String()
		},
		"formatLogs": func(logData []byte) string {
			return string(logData)
		},
	}
}
