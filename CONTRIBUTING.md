# Contributing to ffuf

Thanks for taking the time to contribute. This file is the short version; the
detailed engineering documentation lives in the wiki.

## Before you start

- ffuf needs Go 1.20 or newer.
- Build: `go build` at the repo root produces the `ffuf` binary.
- Test: `go test -race ./...`. The race detector is what CI gates on, so run it
  before you push.

## Engineering documentation (the wiki)

The [wiki Contributing section](https://github.com/ffuf/ffuf/wiki/Contributing)
documents ffuf from the inside:

- [Overview and architecture](https://github.com/ffuf/ffuf/wiki/Contributing):
  how the packages fit together (the `pkg/ffuf` kernel, the `pkg/engine` scan
  engine, the provider interfaces, and the `assembly.BuildJob` wiring seam).
- [Testing](https://github.com/ffuf/ffuf/wiki/Contributing-Testing): the test
  layers (unit, property, characterization goldens, integration, end-to-end),
  how to run each, and which layer a new test belongs in.
- [Flags and help declaration](https://github.com/ffuf/ffuf/wiki/Contributing-Flags-and-Help):
  read this before adding or changing any CLI flag.

Read the architecture and testing pages before a large change; read the flags
page before touching a flag.

## Submitting a pull request

- Base your branch on the latest `master`.
- Keep each pull request to one logical change.
- Run `go test -race ./...`, and make sure `gofmt` and `go vet` are clean.
- If your change affects help, flag, or config output, regenerate the
  characterization goldens and commit them:
  `go test -run Characterization -update-golden .`
- If this is your first contribution, add your name to `CONTRIBUTORS.md`
  (the file is alphabetically ordered).
- Add a short entry describing your change to `CHANGELOG.md` under the `master`
  section.

## Reporting a security issue

Please do not open a public issue for a security vulnerability in ffuf. Report it
privately through the repository's
[security policy](https://github.com/ffuf/ffuf/security/policy).

Thanks for contributing to ffuf :)
