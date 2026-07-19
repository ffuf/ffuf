# Description

Please add a short description of pull request contents.
If this PR addresses an existing issue, please add the issue number below.

Fixes: #(issue number)

## Checklist

- [ ] Branch is based on the latest `master`.
- [ ] `go test -race ./...` passes, and `gofmt` and `go vet` are clean.
- [ ] If this changes help, flag, or config output, the characterization goldens are regenerated (`go test -run Characterization -update-golden .`) and committed.
- [ ] If this is your first time contributing to ffuf, add your name to `CONTRIBUTORS.md` (alphabetically ordered).
- [ ] Add a short description of the change to `CHANGELOG.md`.

New to the codebase? The [wiki Contributing section](https://github.com/ffuf/ffuf/wiki/Contributing) covers the architecture, and the [Testing page](https://github.com/ffuf/ffuf/wiki/Contributing-Testing) covers the test layers.

Thanks for contributing to ffuf :)
