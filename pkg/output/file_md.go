package output

import (
	"html/template"
	"os"
	"time"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type markdownFileOutput struct {
	CommandLine string
	Time        string
	Results     []Result
}

const (
	markdownTemplate = `# FFUF Report

  Command line : ` + "`{{.CommandLine}}`" + `
  Time: ` + "{{ .Time }}" + `

  | Input | Position | Status Code | Content Length | Content Words | Content Lines |
  | :---- | :------- | :---------- | :------------- | :------------ | :------------ |
  {{range .Results}}| {{ .Input }} | {{ .Position }} | {{ .StatusCode }} | {{ .ContentLength }} | {{ .ContentWords }} | {{ .ContentLines }} |
  {{end}}
	` // The template format is not pretty but follows the markdown guide
)

func writeMarkdown(config *ffuf.Config, res []Result) error {

	ti := time.Now()

	outHTML := htmlFileOutput{
		CommandLine: config.CommandLine,
		Time:        ti.Format(time.RFC3339),
		Results:     res,
	}

	f, err := os.Create(config.OutputFile)
	if err != nil {
		return err
	}
	defer f.Close()

	templateName := "output.md"
	t := template.New(templateName).Delims("{{", "}}")
	t.Parse(markdownTemplate)
	t.Execute(f, outHTML)
	return nil
}
