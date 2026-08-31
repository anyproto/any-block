# AnyBlock

AnyBlock is Anytype's open interchange project. It contains both generations
of the format, the codecs between them, bundle tooling, and conformance data.

- **AnyBlock v1** is the protobuf wire format used by Anytype for models,
  events, changes, and snapshots.
- **AnyBlock v2** is the readable, interoperable import/export and API-base
  format. Its first public `formatVersion` is `2.0`; later grammar revisions
  in this family use `2.1`, `2.2`, and so on.

## Repository layout

```text
format/v1/           v1 protobuf sources, generated Go bindings, conformance data
format/v2/           v2 specification, JSON schemas, examples, conformance data
codec/anyblockjson/  Go codec between v1 snapshots and v2 documents
bundle/              v2 bundle validation and composition
cmd/anyblock/        command-line validation and conversion tools
js/                  future JavaScript/Wasm packages
```

A caller imports the two together — v1 types in, AnyBlock JSON out:

```go
import (
	"github.com/anyproto/any-block/codec/anyblockjson"
	v1 "github.com/anyproto/any-block/format/v1/model"
)
```

`format/v1` and `format/v2` own the wire/JSON format artifacts, `codec/anyblockjson`
is the Go conversion API, and `bundle` and `cmd/anyblock` provide the bundle
and command-line surfaces. `Options.TableColumnHeaders` remains opt-in so
backup output is stable.

The four historical root-level v1 `.proto` paths are retained as generated
compatibility mirrors. Their canonical editable sources live in
`format/v1/proto/`; see [the v1 mirror rules](format/v1/README.md).

Start with [format/v1/README.md](format/v1/README.md) or
[format/v2/README.md](format/v2/README.md). The normative v2 definition is
[format/v2/SPEC.md](format/v2/SPEC.md).

## Development

```sh
go test ./...
go vet ./...
go generate ./codec/anyblockjson ./format/v1
```

The format and codec READMEs list the pinned generator installation commands
used by CI.

Fixture and corpus rules for contributors are in
[FIXTURE_POLICY.md](FIXTURE_POLICY.md).

## Contribution

Thank you for your desire to develop Anytype together!

❤️ This project and everyone involved in it is governed by the [Code of Conduct](https://github.com/anyproto/.github/blob/main/docs/CODE_OF_CONDUCT.md).

🧑‍💻 Check out our [contributing guide](https://github.com/anyproto/.github/blob/main/docs/CONTRIBUTING.md) to learn about asking questions, creating issues, or submitting pull requests.

🫢 For security findings, please email [security@anytype.io](mailto:security@anytype.io) and refer to our [security guide](https://github.com/anyproto/.github/blob/main/docs/SECURITY.md) for more information.

🤝 Follow us on [Github](https://github.com/anyproto) and join the [Contributors Community](https://github.com/orgs/anyproto/discussions).

---
Made by Any — a Swiss association 🇨🇭

Licensed under [MIT](./LICENSE.md).
