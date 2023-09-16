## Changelog
- master
  - New
  - Changed
  
- v2.1.0
  - New
    - autocalibration-strategy refactored to support extensible strategy configuration
    - New cli flag `-raw` to omit urlencoding for URIs
    - New cli flags `-ck` and `-cc` to enable the use of client side certificate authentication
    - Integration with `github.com/ffuf/pencode` library, added `-enc` cli flag to do various in-fly encodings for input data
  - Changed
    - Fix multiline output
    - Explicitly allow TLS1.0 
    - Fix markdown output file format
    - Fix csv output file format
    - Fixed divide by 0 error when setting rate limit to 0 manually.
    - Automatic brotli and deflate decompression
    - Report if request times out when a time based matcher or filter is active
    - All 2XX status codes are now matched
    - Allow adding "unused" wordlists in config file

- v2.0.0
  - New
    - Added a new, dynamic keyword `FFUFHASH` that generates hash from job configuration and wordlist position to map blind payloads back to the initial request.
    - New command line parameter for searching a hash: `-search FFUFHASH`
    - Data scraper functionality
    - Requests per second rate can be configured in the interactive mode
  - Changed
    - Multiline output prints out alphabetically sorted by keyword
    - Default configuration directories now follow `XDG_CONFIG_HOME` variable (less spam in your home directory)
    - Fixed issue with autocalibration of line & words filter
    - Rate doesn't have initial burst anymore and is more robust in general
    - Sniper mode template parsing fixes
    - Time-based matcher now works properly
    - Proxy URLs are verified to avoid hard to debug issues
    - Made JSON (`-json`) output format take precedence over quiet output mode, to allow JSON output without the banner etc

  
- v1.5.0
  - New
    - New autocalibration options: `-ach`, `-ack` and `-acs`. Revamped the whole autocalibration process
    - Configurable modes for matchers and filters (CLI flags: `fmode` and `mmode`): "and" and "or"
  - Changed
  
- v1.4.1
  - New
  - Changed
    - Fixed a bug with recursion, introduced in the 1.4.0 release
    - Recursion now works better with multiple wordlists, disabling unnecessary wordlists for queued jobs where needed
  
- v1.4.0
  - New
    - Added response time logging and filtering
    - Added a CLI flag to specify TLS SNI value
    - Added full line colors
    - Added `-json` to emit newline delimited JSON output
    - Added 500 Internal Server Error to list of status codes matched by default
  - Changed
    - Fixed an issue where output file was created regardless of `-or`
    - Fixed an issue where output (often a lot of it) would be printed after entering interactive mode
    - Fixed an issue when reading wordlist files from ffufrc
    - Fixed an issue where `-of all` option only creates one output file (instead of all formats) 
    - Fixed an issue where redirection to the same domain in recursive mode dropped port info from URL
    - Added HTTP2 support

- v1.3.1
  - New
    - Added a CLI flag to disable the interactive mode
  - Changed
    - Do not read the last newline in the end of the raw request file when using -request
    - Fixed an issue with storing the matches for recursion jobs
    - Fixed the way the "size" is calculated, it should match content-length now
    - Fixed an issue with header canonicalization when a keyword was just a part of the header name  
    - Fixed output writing so it doesn't silently fail if it needs to create directories recursively

- v1.3.0
  - New
     - All output file formats now include the `Content-Type`.
     - New CLI flag `-recursion-strategy` that allows adding new queued recursion jobs for non-redirect responses.
     - Ability to enter interactive mode by pressing `ENTER` during  the ffuf execution. The interactive mode allows
    user to change filters, manage recursion queue, save snapshot of matches to a file etc.
  - Changed
    - Fix a badchar in progress output
  
- v1.2.1
  - Changed
    - Fixed a build breaking bug in `input-shell` parameter
    
- v1.2.0
  - New
    - Added 405 Method Not Allowed to list of status codes matched by default.
    - New CLI flag `-rate` to set maximum rate of requests per second. The adjustment is dynamic.
    - New CLI flag `-config` to define a configuration file with preconfigured settings for the job.
    - Ffuf now reads a default configuration file `$HOME/.ffufrc` upon startup. Options set in this file
    are overwritten by the ones provided on CLI.
    - Change banner logging to stderr instead of stdout.
    - New CLI flag `-or` to avoid creating result files if we didn't get any. 
    - New CLI flag `-input-shell` to set the shell to be used by `input-cmd`

  - Changed
    - Pre-flight errors are now displayed also after the usage text to prevent the need to scroll through backlog.
    - Cancelling via SIGINT (Ctrl-C) is now more responsive
    - Fixed issue where a thread would hang due to TCP errors
    - Fixed the issue where the option -ac was overwriting existing filters. Now auto-calibration will add them where needed.
    - The `-w` flag now accepts comma delimited values in the form of `file1:W1,file2:W2`.
    - Links in the HTML report are now clickable
    - Fixed panic during wordlist flag parsing in Windows systems.

- v1.1.0
  - New
    - New CLI flag `-maxtime-job` to set max. execution time per job.
    - Changed behaviour of `-maxtime`, can now be used for entire process.
    - A new flag `-ignore-body` so ffuf does not fetch the response content. Default value=false.
    - Added the wordlists to the header information.
    - Added support to output "all" formats (specify the path/filename sans file extension and ffuf will add the appropriate suffix for the filetype)

  - Changed
    - Fixed a bug related to the autocalibration feature making the random seed initialization also to take place before autocalibration needs it.
    - Added tls renegotiation flag to fix #193 in http.Client
    - Fixed HTML report to display select/combo-box for rows per page (and increased default from 10 to 250 rows).
    - Added Host information to JSON output file
    - Fixed request method when supplying request file
    - Fixed crash with 3XX responses that weren't redirects (304 Not Modified, 300 Multiple Choices etc)

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
