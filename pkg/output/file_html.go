package output

import (
	"html/template"
	"os"
	"time"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type htmlFileOutput struct {
	CommandLine string
	Time        string
	Keys        []string
	Results     []Result
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
|result_raw|StatusCode|Input|Position|ContentLength|ContentWords|ContentLines|
        </div>
          <tr>
              <th>Status</th>
{{ range .Keys }}              <th>{{ . }}</th>
{{ end }}
			  <th>URL</th>
			  <th>Redirect location</th>
              <th>Position</th>
              <th>Length</th>
              <th>Words</th>
              <th>Lines</th>
			  <th>Resultfile</th>
          </tr>
        </thead>

        <tbody>
			{{range $result := .Results}}
                <div style="display:none">
|result_raw|{{ $result.StatusCode }}{{ range $keyword, $value := $result.Input }}|{{ $value | printf "%s" }}{{ end }}|{{ $result.Url }}|{{ $result.RedirectLocation }}|{{ $result.Position }}|{{ $result.ContentLength }}|{{ $result.ContentWords }}|{{ $result.ContentLines }}|
                </div>
                <tr class="result-{{ $result.StatusCode }}" style="background-color: {{$result.HTMLColor}};">
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
                    <td>{{ $result.ResultFile }}</td>
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
func colorizeResults(results []Result) []Result {
	newResults := make([]Result, 0)

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

func writeHTML(config *ffuf.Config, results []Result) error {

  if(config.OutputCreateEmptyFile && (len(results) == 0)){
		return nil
  }
  
	results = colorizeResults(results)

	ti := time.Now()

	keywords := make([]string, 0)
	for _, inputprovider := range config.InputProviders {
		keywords = append(keywords, inputprovider.Keyword)
	}

	outHTML := htmlFileOutput{
		CommandLine: config.CommandLine,
		Time:        ti.Format(time.RFC3339),
		Results:     results,
		Keys:        keywords,
	}

	f, err := os.Create(config.OutputFile)
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
