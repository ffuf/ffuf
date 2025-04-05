package output

import (
	"html"
	"html/template"
	"os"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type htmlResult struct {
	Input            map[string]string
	Position         int
	StatusCode       int64
	ContentLength    int64
	ContentWords     int64
	ContentLines     int64
	ContentType      string
	RedirectLocation string
	ScraperData      string
	Duration         time.Duration
	ResultFile       string
	Url              string
	Host             string
	HTMLColor        string
	FfufHash         string
}

type htmlFileOutput struct {
	CommandLine string
	Time        string
	Keys        []string
	Results     []htmlResult
}

const (
	htmlTemplate = `
<!DOCTYPE html>
<html>
  <head>
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
    <meta
      name="viewport"
      content="width=device-width, initial-scale=1, maximum-scale=1.0"
    />
    <title>FFUF Report - </title>

    <!-- CSS  -->
    <link
      href="https://fonts.googleapis.com/icon?family=Material+Icons"
      rel="stylesheet"
    />
    <link
      rel="stylesheet"
      href="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/css/materialize.min.css"
	/>
	<link 
	  rel="stylesheet" 
	  type="text/css" 
	  href="https://cdn.datatables.net/1.10.20/css/jquery.dataTables.css"
	/>
  
  </head>

  <body>
    <nav>
      <div class="nav-wrapper">
        <a href="#" class="brand-logo">FFUF</a>
        <ul id="nav-mobile" class="right hide-on-med-and-down">
        </ul>
      </div>
    </nav>

    <main class="section no-pad-bot" id="index-banner">
      <div class="container">
        <br /><br />
        <h1 class="header center ">FFUF Report</h1>
        <div class="row center">

		<pre>{{ .CommandLine }}</pre>
		<pre>{{ .Time }}</pre>

   <table id="ffufreport">
        <thead>
        <div style="display:none">
|result_raw|StatusCode{{ range $keyword := .Keys }}|{{ $keyword | printf "%s" }}{{ end }}|Url|RedirectLocation|Position|ContentLength|ContentWords|ContentLines|ContentType|Duration|Resultfile|ScraperData|FfufHash|
        </div>
          <tr>
              <th>Status</th>
{{ range .Keys }}              <th>{{ . }}</th>{{ end }}
			  <th>URL</th>
			  <th>Redirect location</th>
              <th>Position</th>
              <th>Length</th>
              <th>Words</th>
			  <th>Lines</th>
			  <th>Type</th>
              <th>Duration</th>
			  <th>Resultfile</th>
              <th>Scraper data</th>
              <th>Ffuf Hash</th>
          </tr>
        </thead>

        <tbody>
			{{range $result := .Results}}
                <div style="display:none">
|result_raw|{{ $result.StatusCode }}{{ range $keyword, $value := $result.Input }}|{{ $value | printf "%s" }}{{ end }}|{{ $result.Url }}|{{ $result.RedirectLocation }}|{{ $result.Position }}|{{ $result.ContentLength }}|{{ $result.ContentWords }}|{{ $result.ContentLines }}|{{ $result.ContentType }}|{{ $result.Duration }}|{{ $result.ResultFile }}|{{ $result.ScraperData }}|{{ $result.FfufHash }}|
                </div>
                <tr class="result-{{ $result.StatusCode }}" style="background-color: {{ $result.HTMLColor }};">
                    <td><font color="black" class="status-code">{{ $result.StatusCode }}</font></td>
                    {{ range $keyword, $value := $result.Input }}
                        <td>{{ $value | printf "%s" }}</td>
                    {{ end }}
                    <td><a href="{{ $result.Url }}">{{ $result.Url }}</a></td>
                    <td><a href="{{ $result.RedirectLocation }}">{{ $result.RedirectLocation }}</a></td>
                    <td>{{ $result.Position }}</td>
                    <td>{{ $result.ContentLength }}</td>
                    <td>{{ $result.ContentWords }}</td>
					<td>{{ $result.ContentLines }}</td>
					<td>{{ $result.ContentType }}</td>
					<td>{{ $result.Duration }}</td>
                    <td>{{ $result.ResultFile }}</td>
					<td>{{ $result.ScraperData }}</td>
					<td>{{ $result.FfufHash }}</td>
                </tr>
            {{ end }}
        </tbody>
      </table>

        </div>
        <br /><br />
      </div>
    </main>

    <!--JavaScript at end of body for optimized loading-->
	<script src="https://code.jquery.com/jquery-3.4.1.min.js" integrity="sha256-CSXorXvZcTkaix6Yvo6HppcZGetbYMGWSFlBw8HfCJo=" crossorigin="anonymous"></script>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/js/materialize.min.js"></script>
    <script type="text/javascript" charset="utf8" src="https://cdn.datatables.net/1.10.20/js/jquery.dataTables.js"></script>
    <script>
    $(document).ready(function() {
        $('#ffufreport').DataTable(
            {
                "aLengthMenu": [
                    [250, 500, 1000, 2500, -1],
                    [250, 500, 1000, 2500, "All"]
                ]
            }
        )
        $('select').formSelect();
        });
    </script>
    <style>
      body {
        display: flex;
        min-height: 100vh;
        flex-direction: column;
      }

      main {
        flex: 1 0 auto;
      }
    </style>
  </body>
</html>

	`
)

// colorizeResults returns a new slice with HTMLColor attribute
func colorizeResults(results []ffuf.Result) []ffuf.Result {
	newResults := make([]ffuf.Result, 0)

	for _, r := range results {
		result := r
		result.HTMLColor = "black"

		s := result.StatusCode

		if s >= 200 && s <= 299 {
			result.HTMLColor = "#adea9e"
		}

		if s >= 300 && s <= 399 {
			result.HTMLColor = "#bbbbe6"
		}

		if s >= 400 && s <= 499 {
			result.HTMLColor = "#d2cb7e"
		}

		if s >= 500 && s <= 599 {
			result.HTMLColor = "#de8dc1"
		}

		newResults = append(newResults, result)
	}

	return newResults
}

func writeHTML(filename string, config *ffuf.Config, results []ffuf.Result) error {
	results = colorizeResults(results)

	ti := time.Now()

	keywords := make([]string, 0)
	for _, inputprovider := range config.InputProviders {
		keywords = append(keywords, inputprovider.Keyword)
	}
	htmlResults := make([]htmlResult, 0)

	for _, r := range results {
		ffufhash := ""
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
				strscraper = strscraper + "<p><b>" + html.EscapeString(k) + ":</b><br />"
				firstval := true
				for _, val := range v {
					if !firstval {
						strscraper += "<br />"
					}
					strscraper += html.EscapeString(val)
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
			HTMLColor:        r.HTMLColor,
			FfufHash:         ffufhash,
		}
		htmlResults = append(htmlResults, hres)
	}
	outHTML := htmlFileOutput{
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

	templateName := "output.html"
	t := template.New(templateName).Delims("{{", "}}")
	_, err = t.Parse(htmlTemplate)
	if err != nil {
		return err
	}
	err = t.Execute(f, outHTML)
	return err
}
