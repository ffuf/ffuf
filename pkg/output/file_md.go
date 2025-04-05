package output

import (
	"html/template"
	"os"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

const (
	markdownTemplate = `# FFUF Report

  Command line : ` + "`{{.CommandLine}}`" + `
  Time: ` + "{{ .Time }}" + `

  {{ range .Keys }}| {{ . }} {{ end }}| URL | Redirectlocation | Position | Status Code | Content Length | Content Words | Content Lines | Content Type | Duration | ResultFile | ScraperData | Ffufhash
  {{ range .Keys }}| :- {{ end }}| :-- | :--------------- | :---- | :------- | :---------- | :------------- | :------------ | :--------- | :----------- | :------------ | :-------- |
  {{range .Results}}{{ range $keyword, $value := .Input }}| {{ $value | printf "%s" }} {{ end }}| {{ .Url }} | {{ .RedirectLocation }} | {{ .Position }} | {{ .StatusCode }} | {{ .ContentLength }} | {{ .ContentWords }} | {{ .ContentLines }} | {{ .ContentType }} | {{ .Duration}} | {{ .ResultFile }} | {{ .ScraperData }} | {{ .FfufHash }}
  {{end}}` // The template format is not pretty but follows the markdown guide
)

func writeMarkdown(filename string, config *ffuf.Config, results []ffuf.Result) error {
	ti := time.Now()

	keywords := make([]string, 0)
	for _, inputprovider := range config.InputProviders {
		keywords = append(keywords, inputprovider.Keyword)
	}

	htmlResults := make([]htmlResult, 0)

	ffufhash := ""
	for _, r := range results {
		strinput := make(map[string]string)
		for k, v := range r.Input {
			if k == "FFUFHASH" {
				ffufhash = string(v)
			} else {
				strinput[k] = string(v)
			}
		}
		strscraper := ""
		for k, v := range r.ScraperData {
			if len(v) > 0 {
				strscraper = strscraper + "<p><b>" + k + ":</b><br />"
				firstval := true
				for _, val := range v {
					if !firstval {
						strscraper += "<br />"
					}
					strscraper += val
					firstval = false
				}
				strscraper += "</p>"
			}
		}
		hres := htmlResult{
			Input:            strinput,
			Position:         r.Position,
			StatusCode:       r.StatusCode,
			ContentLength:    r.ContentLength,
			ContentWords:     r.ContentWords,
			ContentLines:     r.ContentLines,
			ContentType:      r.ContentType,
			RedirectLocation: r.RedirectLocation,
			ScraperData:      strscraper,
			Duration:         r.Duration,
			ResultFile:       r.ResultFile,
			Url:              r.Url,
			Host:             r.Host,
			FfufHash:         ffufhash,
		}
		htmlResults = append(htmlResults, hres)
	}

	outMD := htmlFileOutput{
		CommandLine: config.CommandLine,
		Time:        ti.Format(time.RFC3339),
		Results:     htmlResults,
		Keys:        keywords,
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	templateName := "output.md"
	t := template.New(templateName).Delims("{{", "}}")
	_, err = t.Parse(markdownTemplate)
	if err != nil {
		return err
	}
	err = t.Execute(f, outMD)
	return err
}
