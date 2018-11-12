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
Usage of ./ffuf:
  -H "Name: Value"
    	Header "Name: Value", separated by colon. Multiple -H flags are accepted.
  -X string
    	HTTP method to use. (default "GET")
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
  -k	Skip TLS identity verification (insecure)
  -mc string
    	Match HTTP status codes from respose (default "200,204,301,302,307,401")
  -mr string
    	Match regexp
  -ms string
    	Match HTTP response size
  -mw string
    	Match amount of words in response
  -s	Do not print additional information (silent mode)
  -t int
    	Number of concurrent threads. (default 40)
  -u string
    	Target URL
  -w string
    	Wordlist path
```
eg. `ffuf -u https://example.org/FUZZ -w /path/to/wordlist`

## Installation

 - [Download](https://github.com/ffuf/ffuf/releases/latest) a prebuilt binary from [releases page](https://github.com/ffuf/ffuf/releases/latest), unpack and run!
 or
 - If you have go compiler installed: `go get github.com/ffuf/ffuf`

## TODO
 - Tests!
 - Option to follow redirects
 - Optional scope for redirects
 - Client / server architecture to queue jobs and fetch the results later
 - Fuzzing multiple values at the same time
 - Output module for file writing in different formats: csv, json
 - Output module to push the results to an HTTP API
