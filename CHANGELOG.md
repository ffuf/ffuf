## Changelog

- master
  - New
  - Changed

- v1.0.2
  - Changed
    - Write POST request data properly to file when ran with `-od`.
    - Fixed a bug by using header canonicaliztion related to HTTP headers being case insensitive.
    - Properly handle relative redirect urls with `-recursion`
    - Calculate req/sec correctly for when using recursion
    - When `-request` is used, allow the user to override URL using `-u`

- v1.0.1
  - Changed
    - Fixed a bug where regex matchers and filters would fail if `-od` was used to store the request & response contents.

- v1.0
  - New
    - New CLI flag `-ic` to ignore comments from wordlist.
    - New CLI flags `-request` to specify the raw request file to build the actual request from and `-request-proto` to define the new request format.
    - New CLI flag `-od` (output directory) to enable writing requests and responses for matched results to a file for postprocessing or debugging purposes.
    - New CLI flag `-maxtime` to limit the running time of ffuf
    - New CLI flags `-recursion` and `-recursion-depth` to control recursive ffuf jobs if directories are found. This requires the `-u` to end with FUZZ keyword.
    - New CLI flag `-replay-proxy` to replay matched requests using a custom proxy.
  - Changed
    - Limit the use of `-e` (extensions) to a single keyword: FUZZ
    - Regexp matching and filtering (-mr/-fr) allow using keywords in patterns
    - Take 429 responses into account when -sa (stop on all error cases) is used
    - Remove -k flag support, convert to dummy flag #134
    - Write configuration to output JSON
    - Better help text.
    - If any matcher is set, ignore -mc default value.

- v0.12
  - New
    - Added a new flag to select a multi wordlist operation mode: `--mode`, possible values: `clusterbomb` and `pitchfork`.
    - Added a new output file format eJSON, for always base64 encoding the input data.
    - Redirect location is always shown in the output files (when using `-o`)
    - Full URL is always shown in the output files (when using `-o`)
    - HTML output format got [DataTables](https://datatables.net/) support allowing realtime searches, sorting by column etc.
    - New CLI flag `-v` for verbose output. Including full URL, and redirect location.
    - SIGTERM monitoring, in order to catch keyboard interrupts an such, to be able to write `-o` files before exiting.
  - Changed
    - Fixed a bug in the default multi wordlist mode
    - Fixed JSON output regression, where all the input data was always encoded in base64
    - `--debug-log` no correctly logs connection errors
    - Removed `-l` flag in favor of `-v`
    - More verbose information in banner shown in startup.

- v0.11
  - New

    - New CLI flag: -l, shows target location of redirect responses
    - New CLI flac: -acc, custom auto-calibration strings
    - New CLI flag: -debug-log, writes the debug logging to the specified file.
    - New CLI flags -ml and -fl, filters/matches line count in response
    - Ability to use multiple wordlists / keywords by defining multiple -w command line flags. The if no keyword is defined, the default is FUZZ to keep backwards compatibility. Example: `-w "wordlists/custom.txt:CUSTOM" -H "RandomHeader: CUSTOM"`.

  - Changed
    - New CLI flag: -i, dummy flag that does nothing. for compatibility with copy as curl.
    - New CLI flag: -b/--cookie, cookie data for compatibility with copy as curl.
    - New Output format are available: HTML and Markdown table.
    - New CLI flag: -l, shows target location of redirect responses
    - Filtering and matching by status code, response size or word count now allow using ranges in addition to single values
    - The internal logging information to be discarded, and can be written to a file with the new `-debug-log` flag.

- v0.10
  - New
    - New CLI flag: -ac to autocalibrate response size and word filters based on few preset URLs.
    - New CLI flag: -timeout to specify custom timeouts for all HTTP requests.
    - New CLI flag: --data for compatibility with copy as curl functionality of browsers.
    - New CLI flag: --compressed, dummy flag that does nothing. for compatibility with copy as curl.
    - New CLI flags: --input-cmd, and --input-num to handle input generation using external commands. Mutators for example. Environment variable FFUF_NUM will be updated on every call of the command.
    - When --input-cmd is used, display position instead of the payload in results. The output file (of all formats) will include the payload in addition to the position however.

  - Changed
    - Wordlist can also be read from standard input
    - Defining -d or --data implies POST method if -X doesn't set it to something else than GET

- v0.9
  - New
    - New output file formats: CSV and eCSV (CSV with base64 encoded input field to avoid CSV breakage with payloads containing a comma)
    - New CLI flag to follow redirects
    - Erroring connections will be retried once
    - Error counter in status bar
    - New CLI flags: -se (stop on spurious errors) and -sa (stop on all errors, implies -se and -sf)
    - New CLI flags: -e to provide a list of extensions to add to wordlist entries, and -D to provide DirSearch wordlist format compatibility.
    - Wildcard option for response status code matcher.
- v0.8
  - New
    - New CLI flag to write output to a file in JSON format
    - New CLI flag to stop on spurious 403 responses
  - Changed
    - Regex matching / filtering now matches the headers alongside of the response body
