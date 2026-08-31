# Fixture and corpus sanitization policy

## Public-fixture rule

Repository fixtures must be invented, deterministic, minimal, and visibly
synthetic. Production measurements may supply aggregate counts and structural
facts (length, encoding, field presence, nesting, or frequency), but raw user
documents, account identifiers, invite material, request metadata, analytics
identifiers, file keys, content addresses, access tokens, and personal contact
data must not be copied into this repository.

Shape-sensitive test values live in `internal/testfixtures`. Account
identities are derived at test time from fixed byte sequences with a valid
version and checksum. Other sensitive-looking values contain `SYNTHETIC`, or
use an obvious repeated-character CID-shaped sentinel. Public explanatory
documents use the same narrow convention: a repeated-payload CID or an
explicitly documented 48-character `A`-plus-repeated-`1` account sentinel.
Tests assert the historical lengths so sanitization cannot weaken the behavior
under test.

Conformance examples are authored examples, not corpus exports. Aggregate
corpus statements in comments and format documents are retained because they
contain counts and conclusions rather than records or identifiers.

## Automated gate

`TestRepositoryFixtureSecretScan` enumerates the current public tree on every
`go test ./...` run. Every regular file is deliberately classified as UTF-8
text or binary. Every textual path is scanned for retired and near-retired literals,
48-character account identities, email addresses, CIDv0/CIDv1 addresses,
private-key and current token prefixes, and high-entropy base58/base64 values
in credential/key/invite/request contexts. Binary bytes are skipped only after
that explicit classification.

Exemptions are narrow and semantic: `@example.com`; the project's own
published role addresses, as a literal allowlist rather than a domain rule, so
an individual's mailbox at the same domain is still caught; values containing
the literal marker `SYNTHETIC`; and documented repeated-payload account/CID
sentinels. A path such as
`examples`, `conformance`, or `_test.go` is never itself an exemption.
`TestSyntheticFixtureShapes` checks the deterministic identity checksum and
every security-sensitive length.

Suspected exposure must be removed from history or rotated in its owning
system as appropriate; merely adding a scanner exception is not clearance.
