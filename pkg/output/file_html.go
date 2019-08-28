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
	Results     []Result
}

const (
	myTemplate = `
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

   <table>
        <thead>
          <tr>
              <th>Input</th>
              <th>Position</th>
              <th>Status</th>
              <th>Length</th>
              <th>Words</th>
          </tr>
        </thead>

        <tbody>
            {{range .Results}}
                <tr>
                  <td>{{ .Input }}</td>
                  <td>{{ .Position }}</td>
                  <td>{{ .StatusCode }}</td>
                  <td>{{ .ContentLength }}</td>
                  <td>{{ .ContentWords }}</td>
                </tr>
            {{end}}

        </tbody>
      </table>

        </div>
        <br /><br />
      </div>
    </main>

    <!--JavaScript at end of body for optimized loading-->
    <script src="https://cdnjs.cloudflare.com/ajax/libs/materialize/1.0.0/js/materialize.min.js"></script>
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

func writeHTML(config *ffuf.Config, res []Result) error {

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

	templateName := "output.html"
	t := template.New(templateName).Delims("{{", "}}")
	t.Parse(myTemplate)
	t.Execute(f, outHTML)
	return nil
}
