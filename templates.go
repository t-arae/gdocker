package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"os"
	"strings"
)

type dataMakeHeader struct {
	Name string
	Tags []string
}

type dataMakeResource = struct {
	Tag      string
	Resource string
	Commands []string
}

type dataMakeOldVer = struct {
	Tag       string
	Resources []string
}

type BoxedWriter struct {
	Title string
	Out   io.Writer
}

func (bw *BoxedWriter) Write(p []byte) (n int, err error) {
	content := strings.ReplaceAll(string(p), "\t", "   ") // Replace tabs with spaces
	lines := strings.Split(content, "\n")

	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	dashLen := maxLen - len(bw.Title) - 4
	if dashLen < 0 {
		dashLen = 0
	}
	titleLine := fmt.Sprintf("+-- %s %s+", bw.Title, strings.Repeat("-", dashLen))

	var boxedLines []string
	for _, line := range lines {
		padding := strings.Repeat(" ", maxLen-len(line))
		boxedLines = append(boxedLines, fmt.Sprintf("|%s%s|", line, padding))
	}

	bottomLine := "+" + strings.Repeat("-", maxLen) + "+"

	boxed := append([]string{titleLine}, boxedLines...)
	boxed = append(boxed, bottomLine)
	full := "\n" + strings.Join(boxed, "\n") + "\n"

	_, err = io.WriteString(bw.Out, full)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// appendBuffer appends the rendered template to the provided buffer.
func appendBuffer(buf *bytes.Buffer, tmpl string, data any) {
	parsed, err := template.New("").Delims("{{<", ">}}").Parse(tmpl)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	err = parsed.Execute(buf, data)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

type Templates struct {
	templates []string
	tmplsdata []any
}

func NewTemplates(t string, data any) *Templates {
	return &Templates{
		templates: []string{t},
		tmplsdata: []any{data},
	}
}

func (ts *Templates) AddTemplate(t string, data any) {
	ts.templates = append(ts.templates, t)
	ts.tmplsdata = append(ts.tmplsdata, data)
}

func (ts *Templates) writeTemplates(file string, box bool) {
	var err error
	var buf bytes.Buffer

	for i, t := range ts.templates {
		data := ts.tmplsdata[i]
		if data == nil {
			data = struct{}{} // Use an empty struct if no data is provided
		}
		appendBuffer(&buf, t, data)
	}

	var w io.Writer
	if file == "stdout" {
		w = os.Stdout
	} else {
		if box {
			w = &BoxedWriter{Title: file, Out: os.Stdout}
		} else {
			var f *os.File
			f, err = os.Create(file)
			if err != nil {
				slog.Error(err.Error())
				os.Exit(1)
			}
			defer f.Close()
			w = f
		}
	}
	_, err = w.Write(buf.Bytes())
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
