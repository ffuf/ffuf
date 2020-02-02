```
        /'___\  /'___\           /'___\
       /\ \__/ /\ \__/  __  __  /\ \__/
       \ \ ,__\\ \ ,__\/\ \/\ \ \ \ ,__\
        \ \ \_/ \ \ \_/\ \ \_\ \ \ \ \_/
         \ \_\   \ \_\  \ \____/  \ \_\
          \/_/    \/_/   \/___/    \/_/
```

# ffuf - Fuzz Faster U Fool

A fast web fuzzer written in Go.

## Installation

- [Download](https://github.com/ffuf/ffuf/releases/latest) a prebuilt binary from [releases page](https://github.com/ffuf/ffuf/releases/latest), unpack and run!
  or
- If you have go compiler installed: `go get github.com/ffuf/ffuf`

The only dependency of ffuf is Go 1.11. No dependencies outside of Go standard library are needed.

## Example usage

### Typical directory discovery

[![asciicast](https://asciinema.org/a/211350.png)](https://asciinema.org/a/211350)

By using the FUZZ keyword at the end of URL (`-u`):

```
ffuf -w /path/to/wordlist -u https://target/FUZZ
```

### Virtual host discovery (without DNS records)

[![asciicast](https://asciinema.org/a/211360.png)](https://asciinema.org/a/211360)

Assuming that the default virtualhost response size is 4242 bytes, we can filter out all the responses of that size (`-fs 4242`)while fuzzing the Host - header:

```
ffuf -w /path/to/vhost/wordlist -u https://target -H "Host: FUZZ" -fs 4242
```

### GET parameter fuzzing

GET parameter name fuzzing is very similar to directory discovery, and works by defining the `FUZZ` keyword as a part of the URL. This also assumes an response size of 4242 bytes for invalid GET parameter name.

```
ffuf -w /path/to/paramnames.txt -u https://target/script.php?FUZZ=test_value -fs 4242
```

If the parameter name is known, the values can be fuzzed the same way. This example assumes a wrong parameter value returning HTTP response code 401.

```
ffuf -w /path/to/values.txt -u https://target/script.php?valid_name=FUZZ -fc 401
```

### POST data fuzzing

This is a very straightforward operation, again by using the `FUZZ` keyword. This example is fuzzing only part of the POST request. We're again filtering out the 401 responses.

```
ffuf -w /path/to/postdata.txt -X POST -d "username=admin\&password=FUZZ" -u https://target/login.php -fc 401
```

### Using external mutator to produce test cases

For this example, we'll fuzz JSON data that's sent over POST. [Radamsa](https://gitlab.com/akihe/radamsa) is used as the mutator.

When `--input-cmd` is used, ffuf will display matches as their position. This same position value will be available for the callee as an environment variable `$FFUF_NUM`. We'll use this position value as the seed for the mutator. Files example1.txt and example2.txt contain valid JSON payloads. We are matching all the responses, but filtering out response code `400 - Bad request`:

```
ffuf --input-cmd 'radamsa --seed $FFUF_NUM example1.txt example2.txt' -H "Content-Type: application/json" -X POST -u https://ffuf.io.fi/ -mc all -fc 400
```

It of course isn't very efficient to call the mutator for each payload, so we can also pre-generate the payloads, still using [Radamsa](https://gitlab.com/akihe/radamsa) as an example:

```
# Generate 1000 example payloads
radamsa -n 1000 -o %n.txt example1.txt example2.txt

# This results into files 1.txt ... 1000.txt
# Now we can just read the payload data in a loop from file for ffuf

ffuf --input-cmd 'cat $FFUF_NUM.txt' -H "Content-Type: application/json" -X POST -u https://ffuf.io.fi/ -mc all -fc 400
```

## Usage

To define the test case for ffuf, use the keyword `FUZZ` anywhere in the URL (`-u`), headers (`-H`), or POST data (`-d`).

```
Fuzz Faster U Fool - v1.0

HTTP OPTIONS:
  -H               Header `"Name: Value"`, separated by colon. Multiple -H flags are accepted.
  -X               HTTP method to use (default: GET)
  -b               Cookie data `"NAME1=VALUE1; NAME2=VALUE2"` for copy as curl functionality.
  -d               POST data
  -r               Follow redirects (default: false)
  -recursion       Scan recursively. Only FUZZ keyword is supported, and URL (-u) has to end in it. (default: false)
  -recursion-depth Maximum recursion depth. (default: 0)
  -replay-proxy    Replay matched requests using this proxy.
  -timeout         HTTP request timeout in seconds. (default: 10)
  -u               Target URL
  -x               HTTP Proxy URL

GENERAL OPTIONS:
  -V               Show version information. (default: false)
  -ac              Automatically calibrate filtering options (default: false)
  -acc             Custom auto-calibration string. Can be used multiple times. Implies -ac
  -c               Colorize output. (default: false)
  -maxtime         Maximum running time in seconds. (default: 0)
  -p               Seconds of `delay` between requests, or a range of random delay. For example "0.1" or "0.1-2.0"
  -s               Do not print additional information (silent mode) (default: false)
  -sa              Stop on all error cases. Implies -sf and -se. (default: false)
  -se              Stop on spurious errors (default: false)
  -sf              Stop when > 95% of responses return 403 Forbidden (default: false)
  -t               Number of concurrent threads. (default: 40)
  -v               Verbose output, printing full URL and redirect location (if any) with the results. (default: false)

MATCHER OPTIONS:
  -mc              Match HTTP status codes, or "all" for everything. (default: 200,204,301,302,307,401,403)
  -ml              Match amount of lines in response
  -mr              Match regexp
  -ms              Match HTTP response size
  -mw              Match amount of words in response

FILTER OPTIONS:
  -fc              Filter HTTP status codes from response. Comma separated list of codes and ranges
  -fl              Filter by amount of lines in response. Comma separated list of line counts and ranges
  -fr              Filter regexp
  -fs              Filter HTTP response size. Comma separated list of sizes and ranges
  -fw              Filter by amount of words in response. Comma separated list of word counts and ranges

INPUT OPTIONS:
  -D               DirSearch wordlist compatibility mode. Used in conjunction with -e flag. (default: false)
  -e               Comma separated list of extensions. Extends FUZZ keyword.
  -ic              Ignore wordlist comments (default: false)
  -input-cmd       Command producing the input. --input-num is required when using this input method. Overrides -w.
  -input-num       Number of inputs to test. Used in conjunction with --input-cmd. (default: 100)
  -mode            Multi-wordlist operation mode. Available modes: clusterbomb, pitchfork (default: clusterbomb)
  -request         File containing the raw http request
  -request-proto   Protocol to use along with raw request (default: https)
  -w               Wordlist file path and (optional) keyword separated by colon. eg. '/path/to/wordlist:KEYWORD'

OUTPUT OPTIONS:
  -debug-log       Write all of the internal logging to the specified file.
  -o               Write output to file
  -od              Directory path to store matched results to.
  -of              Output file format. Available formats: json, ejson, html, md, csv, ecsv (default: json)

EXAMPLE USAGE:
  Fuzz file paths from wordlist.txt, match all responses but filter out those with content-size 42.
  Colored, verbose output.
    ffuf -w wordlist.txt -u https://example.org/FUZZ -mc all -fs 42 -c -v

  Fuzz Host-header, match HTTP 200 responses.
    ffuf -w hosts.txt -u https://example.org/ -H "Host: FUZZ" -mc 200

  Fuzz POST JSON data. Match all responses not containing text "error".
    ffuf -w entries.txt -u https://example.org/ -X POST -H "Content-Type: application/json" \
      -d '{"name": "FUZZ", "anotherkey": "anothervalue"}' -fr "error"

  Fuzz multiple locations. Match only responses reflecting the value of "VAL" keyword. Colored.
    ffuf -w params.txt:PARAM -w values.txt:VAL -u https://example.org/?PARAM=VAL -mr "VAL" -c

  More information and examples: https://github.com/ffuf/ffuf
```

## Helper scripts and advanced payloads

See [ffuf-scripts](https://github.com/ffuf/ffuf-scripts) repository for helper scripts and payload generators
for different workflows and usage scenarios.

## License

ffuf is released under MIT license. See [LICENSE](https://github.com/ffuf/ffuf/blob/master/LICENSE).
