# Contributing

Thanks for taking an interest. tgscreens is small on purpose, so the best contributions fix a bug,
sharpen the docs, or add something that every state-machine bot needs.

## Before you start

Open an issue before a large change, especially one that touches the public API or adds a
dependency. Each change is weighed against the cost to existing bots.

Keep the scope in mind: the router knows nothing about any bot's domain or storage, and the
`telegram` package knows nothing about routing. A change that makes either know more than that is
unlikely to fit.

## Development

```bash
go vet ./...
go test -race ./...
go build ./examples/...
```

CI runs the same three commands. The race detector needs cgo, so on a machine without a C
toolchain run `go test ./...` and let CI cover `-race`.

## Guidelines

- Format with `gofmt`, and document every exported identifier.
- Add or update a test with each behaviour change. Flows run through `Listen` with a mock `Sender`
  and a closure `Source`, as in [`examples/echo/main_test.go`](examples/echo/main_test.go).
- If you change `telegram.Sender`, regenerate the mock with `go generate ./...` and commit it.
- Docs are part of the change. Every `go` code block in `README.md` and `docs/` must be a verbatim
  excerpt of compiled source in `examples/` or a test file; `TestDocs` fails otherwise. Use a
  `text` block for anything that is not real code.
- Keep callback data short. Anything that can grow with user input belongs behind a `Hash`.

## Pull requests

Keep a pull request to one concern, describe the behaviour it changes and why, and make sure CI
passes. By contributing you agree that your work is licensed under the [MIT license](LICENSE).
