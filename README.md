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

Heavily inspired by the great projects [gobuster](https://github.com/OJ/gobuster) and [wfuzz](https://github.com/xmendez/wfuzz).

## Features

 - Fast!
 - Allows fuzzing of HTTP header values, POST data, and different parts of URL, including GET parameter names and values
 - Silent mode (`-s`) for clean output that's easy to use in pipes to other processes.
 - Modularized architecture that allows integration with existing toolchains with reasonable effort
 - Easy-to-add filters and matchers (they are interoperable)

## Example cases

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
ffuf -w /path/to/postdata.txt -X POST -d "username=admin\&password=FUZZ" https://target/login.php -fc 401
```

## Usage

To define the test case for ffuf, use the keyword `FUZZ` anywhere in the URL (`-u`), headers (`-H`), or POST data (`-d`).
```
  -H "Name: Value"
    	Header "Name: Value", separated by colon. Multiple -H flags are accepted.
  -V	Show version information.
  -X string
    	HTTP method to use (default "GET")
  -c	Colorize output.
  -d string
    	POST data.
  -fc string
    	Filter HTTP status codes from response
  -fr string
    	Filter regexp
  -fs string
    	Filter HTTP response size
  -fw string
    	Filter by amount of words in response
  -k	TLS identity verification
  -mc string
    	Match HTTP status codes from respose (default "200,204,301,302,307,401,403")
  -mr string
    	Match regexp
  -ms string
    	Match HTTP response size
  -mw string
    	Match amount of words in response
  -o string
    	Write output to file
  -of string
    	Output file format. Available formats: json, csv, ecsv (default "json")
  -p delay
    	Seconds of delay between requests, or a range of random delay. For example "0.1" or "0.1-2.0"
  -r	Follow redirects
  -s	Do not print additional information (silent mode)
  -sa
    	Stop on all error cases. Implies -sf and -se
  -se
    	Stop on spurious errors
  -sf
    	Stop when > 90% of responses return 403 Forbidden
  -t int
    	Number of concurrent threads. (default 40)
  -u string
    	Target URL
  -w string
    	Wordlist path
  -x string
    	HTTP Proxy URL
```

eg. `ffuf -u https://example.org/FUZZ -w /path/to/wordlist`

## Installation

 - [Download](https://github.com/ffuf/ffuf/releases/latest) a prebuilt binary from [releases page](https://github.com/ffuf/ffuf/releases/latest), unpack and run!
 or
 - If you have go compiler installed: `go get github.com/ffuf/ffuf`

The only dependency of ffuf is Go 1.11. No dependencies outside of Go standard library are needed.

## Changelog

- master
   - New
      - New output file formats: CSV and eCSV (CSV with base64 encoded input field to avoid CSV breakage with payloads containing a comma)
      - New CLI flag to follow redirects
      - Erroring connections will be retried once
      - Error counter in status bar
      - New CLI flags: -se (stop on spurious errors) and -sa (stop on all errors, implies -se and -sf)
- v0.8
   - New
      - New CLI flag to write output to a file in JSON format
      - New CLI flag to stop on spurious 403 responses
   - Changed
      - Regex matching / filtering now matches the headers alongside of the response body

## TODO
 - Tests!
 - Optional scope for redirects
 - Client / server architecture to queue jobs and fetch the results later
 - Fuzzing multiple values at the same time
 - Output module to push the results to an HTTP API
