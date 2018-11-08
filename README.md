# ffuf - Fuzz Faster U Fool

A fast web fuzzer written in Go, allows fuzzing of URL and header values.

## Usage
```
Usage of ./ffuf:
  -H value
    	Header name and value, separated by colon. Multiple -H flags are accepted.
  -X string
    	HTTP method to use. (default "GET")
  -fc string
    	Filter HTTP status codes from response
  -fs string
    	Filter HTTP response size
  -k	Skip TLS identity verification (insecure)
  -mc string
    	Match HTTP status codes from respose (default "200,204,301,302,307")
  -ms string
    	Match HTTP response size
  -s	Do not print additional information (silent mode)
  -t int
    	Number of concurrent threads. (default 20)
  -u string
    	Target URL
  -w string
    	Wordlist path
```
eg. `ffuf -u https://example.org/FUZZ -w /path/to/wordlist`

## Installation

Either download a prebuilt binary from Releases page or install [Go 1.9 or newer](https://golang.org/doc/install). and build the project with `go get && go build`
