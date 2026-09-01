package testfixtures

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multibase"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type semanticSecretFinding struct {
	Path string
	Line int
	Kind string
}

func (f semanticSecretFinding) String() string {
	return fmt.Sprintf("%s:%d: %s", f.Path, f.Line, f.Kind)
}

func countFindingKind(findings []semanticSecretFinding, kind string) int {
	count := 0
	for _, finding := range findings {
		if finding.Kind == kind {
			count++
		}
	}
	return count
}

type semanticSecretScan struct {
	TextPaths   []string
	BinaryPaths []string
	Findings    []semanticSecretFinding
}

var credentialSignatures = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"PEM private key", regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)},
	{"AWS long-lived access key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"AWS session access key", regexp.MustCompile(`\bASIA[0-9A-Z]{16}\b`)},
	{"GitHub classic token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`)},
	{"GitHub fine-grained token", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`)},
	{"npm token", regexp.MustCompile(`\bnpm_[A-Za-z0-9]{20,}\b`)},
	{"Stripe live secret key", regexp.MustCompile(`\bsk_live_[A-Za-z0-9]{16,}\b`)},
	{"Google API key", regexp.MustCompile(`\bAIza[A-Za-z0-9_-]{20,}\b`)},
	{"Slack token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`)},
	{"JWT", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)},
}

var (
	emailPattern = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
	// Account identities encode to 48 base58 characters and currently begin
	// with A because of their version byte. Keeping the prefix in this rule
	// avoids treating ordinary 48-character identifiers and prose as accounts.
	accountIdentityPattern     = regexp.MustCompile(`A[1-9A-HJ-NP-Za-km-z]{47}`)
	longTokenPattern           = regexp.MustCompile(`[A-Za-z0-9._+/=-]{20,}`)
	sensitiveAssignmentPattern = regexp.MustCompile(
		`(?i)(?:invite(?:[_ -]?file)?[_ -]?(?:key|token|code)|request(?:[_ -]?metadata)?[_ -]?(?:key|token)|(?:file|encryption|private|api|access|secret)[_ -]?(?:key|token|secret)|password|credential)` +
			`(?:["'\x60])?\s*(?::=|:|=)\s*(?:"""|'''|\$'|["'\x60])?([A-Za-z0-9._+/-]{20,}={0,2})`)
	hexHashPattern       = regexp.MustCompile(`^[0-9a-fA-F]+$`)
	syntheticValueMarker = regexp.MustCompile(`(?:^|[^A-Za-z0-9])SYNTHETIC(?:$|[^A-Za-z0-9])`)
)

// retiredCorpusSignature is a non-reversible exact/fuzzy signature. Exact
// matching uses the full digest. A one-substitution candidate necessarily
// leaves either the left or right half unchanged, so the two partition
// digests retain that detection without storing the retired bytes in source,
// fixtures, or the compiled test binary. Matching a larger change confined to
// one half is an intentional conservative rejection.
type retiredCorpusSignature struct {
	length      int
	exactSHA256 string
	leftSHA256  string
	rightSHA256 string
}

var retiredCorpusSignatures = []retiredCorpusSignature{
	{length: 28, exactSHA256: "427a0169980527097847c9e0b4763e73fd30871faec7e408fb86ec8fbf7a54a0", leftSHA256: "312c8b5cac2d1679fc6f5f6c0e490a987fd1ebe99929b197ec5409d61b3134d4", rightSHA256: "778dc4c2bf8ce1d3d0e3bcd04d88f7621fb9de86159e7b5dc6b79f70d76347bc"},
	{length: 29, exactSHA256: "5ac2f8429e5249bdfb16dde5504803365ecff8ecf94c96ab74cd6e61ec0b1b6c", leftSHA256: "f77b8912ea83ccae16307ea4b3351aa093a669f33eab4136d6f5c190a3bfd2a5", rightSHA256: "903f018da80d9fc0a39fc0482dc5d9dfad923546d9d49286fb8eb557758c0391"},
	{length: 29, exactSHA256: "0e5e1fd5611cf149457ccf3b75f23f29a621d47ea341ecb7282b0dd97e380037", leftSHA256: "2c4a3e7cf1a1e426d09a99784de268c2715025335380e59ef6768c52c6e9c2e5", rightSHA256: "7878f5f43f2f7aa17c84f24c080a5598d25f54072f1baa641dc9cdc372f85047"},
	{length: 29, exactSHA256: "3cb6f4915c808dcb42ddb5eec934af4fe4914ce41125f6abeab2f3e7b2801f25", leftSHA256: "9c6ed1b8f6cdbf8892b20cd34095b46b03556c631589c6004b2476aae1da6238", rightSHA256: "e51112be2fc784df44c70f01d362661b676ca63de7a006e85b0018c938bd55f0"},
	{length: 53, exactSHA256: "a8a2a2a4bfbf068e804a9fbbadbcf010d33ee6cfeea8da4e1a4f2f5cd118ea76", leftSHA256: "e3110a5a9996cc9e7c360f3dcb71a5f3b702e5b91cc861041f30521b43db6661", rightSHA256: "d161ed45ed8cfb647b5717c7d101594f331fae39d4d3ed5bbd5a281638b3c84a"},
	{length: 59, exactSHA256: "0552fbfef7f5fc847a5681a7805d8f2618ed4ea2e7dd4f139950ccb04c4dff10", leftSHA256: "fcd5eca6b050a8e764bbab84b49e4803c2feb48d9473d6b5eef739be4a2e7ef1", rightSHA256: "10d0ea63b0ad3f9f1eb91f97e81693a9e5d27f3444a4facd0d1a0163dfd9ea6c"},
	{length: 52, exactSHA256: "5d299058323cb2dabb873101f633b8b233a8152b7c1e7353c168e2bb4f449c36", leftSHA256: "805e0a6ec8ed534e30f6ad0b4f8a637e62f722e24f994e211e308c49d2a5619c", rightSHA256: "497152fb79b476c0545a14f0d86b20a64376291e9e97f7d921618b3ea0a9e398"},
	{length: 44, exactSHA256: "db0668e9d2c8f9bc66b56c8b225ff4ca7168b64ee79c929484ff73cd741e8dd7", leftSHA256: "0c77c714b09aaaee27ae354c78267cdecef47dbde0a0ac7c95dd2662d449910d", rightSHA256: "52326db262c9fb0c9f961785748ccaa06a0abcc6259027077e4822cd0cf876bb"},
	{length: 59, exactSHA256: "ab93b0b1d7c05476bae8fda1fb69be37fde910a07447dfcc1bb2d0c7e9d0264a", leftSHA256: "04ec623d767e1417beb70eaa76fa6c14f40cebb90ab160fdeeebd13741c299c5", rightSHA256: "c47936439d3ea20ecc218dd62c619ac70861e5342bd1874c3cb87fa642dcaeeb"},
	{length: 48, exactSHA256: "cce3e458b10c7582a4ffe82f8779ba05384a380d0746f001321f68e81b7b4997", leftSHA256: "10fc721f380d2086a195c11882329479d45d11f738e422cc4a202eff8a51ab88", rightSHA256: "81198644c827e8a766f06021f5661de6ba3ea8cd5a1dccda0c3c0e68075011af"},
	{length: 73, exactSHA256: "7b578804a03e4240355f9938a720543e58549812604c683aa07f606b0b8a8bde", leftSHA256: "c7b871f371c5558323fac8318e4f6279e8a72d41f29d50f7584cc3f74692be23", rightSHA256: "38ca077d92bfff1a9b0c7f02d4de89f1e4cac7601e62f88199c99ee83be87149"},
	{length: 48, exactSHA256: "9e99e97d1537f727474757628a3fa228cade37cce1dab32893120b0d94978280", leftSHA256: "fc807563653471c6e889aa4a44d80b42b43e1622862f419efd78962f6a3f6bbd", rightSHA256: "81b65ae939a3f3ca76c542fabee3a408b0840d6f1ed96656e4a82d5b38027f68"},
}

// TestRepositoryFixtureSecretScan enumerates the current regular-file union
// directly, so adding a file cannot evade semantic review.
func TestRepositoryFixtureSecretScan(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))

	got, err := scanRepositoryForSecrets(root)
	require.NoError(t, err)
	for _, finding := range got.Findings {
		t.Errorf("semantic secret scan: %s", finding)
	}

	current, err := enumerateRepositoryFiles(root)
	require.NoError(t, err)
	assert.Equal(t, len(current.paths), len(got.TextPaths)+len(got.BinaryPaths),
		"every current regular source path must be deliberately classified")
	assert.Contains(t, got.TextPaths, "format/v2/schema/object.schema.json",
		"large JSON remains textual even when platform file heuristics call it binary")
}

func scanRepositoryForSecrets(root string) (semanticSecretScan, error) {
	// enumerateRepositoryFiles walks current filesystem state and enforces the
	// shared no-symlink/no-special-file policy.
	current, err := enumerateRepositoryFiles(root)
	if err != nil {
		return semanticSecretScan{}, err
	}
	var result semanticSecretScan
	for _, relative := range current.paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return semanticSecretScan{}, fmt.Errorf("read semantic-scan path %q: %w", relative, err)
		}
		if !isTextualSource(data) {
			result.BinaryPaths = append(result.BinaryPaths, relative)
			continue
		}
		result.TextPaths = append(result.TextPaths, relative)
		result.Findings = append(result.Findings, scanTextualSecrets(relative, string(data))...)
	}
	sort.Slice(result.Findings, func(i, j int) bool {
		if result.Findings[i].Path != result.Findings[j].Path {
			return result.Findings[i].Path < result.Findings[j].Path
		}
		if result.Findings[i].Line != result.Findings[j].Line {
			return result.Findings[i].Line < result.Findings[j].Line
		}
		return result.Findings[i].Kind < result.Findings[j].Kind
	})
	return result, nil
}

// isTextualSource deliberately classifies content rather than trusting a file
// extension. UTF-8 (including ASCII) without NUL bytes is text; invalid UTF-8
// or NUL-bearing content is binary and recorded as such by the caller.
func isTextualSource(data []byte) bool {
	return utf8.Valid(data) && !bytes.ContainsRune(data, '\x00')
}

func scanTextualSecrets(relative, content string) []semanticSecretFinding {
	return scanTextualSecretsWithRetiredSignatures(relative, content, retiredCorpusSignatures)
}

func scanTextualSecretsWithRetiredSignatures(relative, content string, retired []retiredCorpusSignature) []semanticSecretFinding {
	var findings []semanticSecretFinding
	for lineNumber, line := range strings.Split(content, "\n") {
		lineNumber++
		add := func(kind string) {
			findings = append(findings, semanticSecretFinding{Path: relative, Line: lineNumber, Kind: kind})
		}

		exactRetired, nearRetired := retiredMatchesOnLine(line, retired)
		if exactRetired {
			add("retired corpus-derived literal")
		}
		if nearRetired {
			add("near-retired corpus-derived literal")
		}

		for _, signature := range credentialSignatures {
			if signature.re.MatchString(line) {
				add(signature.kind + " signature")
			}
		}

		for _, address := range emailPattern.FindAllString(line, -1) {
			if !isPublishableAddress(address) {
				add("non-example email address")
			}
		}
		for _, candidate := range decodedCIDCandidates(line) {
			if isRepeatedCIDSentinel(candidate.text, candidate.decoded) {
				continue
			}
			add(fmt.Sprintf("non-synthetic CIDv%d content address", candidate.decoded.Version()))
		}
		for _, match := range accountIdentityPattern.FindAllStringIndex(line, -1) {
			if !hasAccountIdentityBoundaries(line, match[0], match[1]) {
				continue
			}
			account := line[match[0]:match[1]]
			if !hasRepeatedPayload(account, 1) && !hasAttachedSyntheticAccountSuffix(line, match[1]) {
				add("non-synthetic 48-character account identity")
			}
		}

		for _, match := range sensitiveAssignmentPattern.FindAllStringSubmatch(line, -1) {
			candidate := match[1]
			if isSafeContextualValue(candidate) {
				continue
			}
			if looksEncodedAndHighEntropy(candidate) {
				add("high-entropy value in a credential/key context")
			}
		}
	}
	return findings
}

func retiredMatchesOnLine(line string, signatures []retiredCorpusSignature) (exact, near bool) {
	for _, token := range longTokenPattern.FindAllString(line, -1) {
		for _, signature := range signatures {
			if len(token) < signature.length {
				continue
			}
			for start := 0; start+signature.length <= len(token); start++ {
				candidate := token[start : start+signature.length]
				candidateExact, candidateNear := signature.matches(candidate)
				exact = exact || candidateExact
				near = near || candidateNear
			}
		}
	}
	return exact, near
}

func (signature retiredCorpusSignature) matches(candidate string) (exact, near bool) {
	if len(candidate) != signature.length {
		return false, false
	}
	if sha256Text(candidate) == signature.exactSHA256 {
		return true, false
	}
	middle := len(candidate) / 2
	return false, sha256Text(candidate[:middle]) == signature.leftSHA256 ||
		sha256Text(candidate[middle:]) == signature.rightSHA256
}

func newRetiredCorpusSignature(value string) retiredCorpusSignature {
	middle := len(value) / 2
	return retiredCorpusSignature{
		length:      len(value),
		exactSHA256: sha256Text(value),
		leftSHA256:  sha256Text(value[:middle]),
		rightSHA256: sha256Text(value[middle:]),
	}
}

func sha256Text(value string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
}

type decodedCIDCandidate struct {
	text    string
	decoded cid.Cid
}

// decodedCIDCandidates discovers candidates using the alphabet selected by
// each multibase prefix. Some structural delimiters are also data in selected
// alphabets: '/' in base64 and '=' in padded encodings. In those cases the
// first parseable prefix ending at a delimiter is the exact CID extent. This
// leaves the delimiter available to bound a following candidate without
// accepting shorter alphanumeric or Unicode embeddings.
func decodedCIDCandidates(line string) []decodedCIDCandidate {
	var candidates []decodedCIDCandidate
	for start := 0; start < len(line); {
		value, size := utf8.DecodeRuneInString(line[start:])
		if hasCIDEmbeddingBefore(line, start) {
			start += size
			continue
		}

		encoding, dataStart, ok := cidTextEncodingAt(line, start, value, size)
		if !ok {
			start += size
			continue
		}
		end := dataStart
		var delimiterEnds []int
		for end < len(line) {
			dataRune, dataSize := utf8.DecodeRuneInString(line[end:])
			if !isMultibaseDataRune(encoding, dataRune) {
				break
			}
			if dataRune == '/' || dataRune == '=' {
				delimiterEnds = append(delimiterEnds, end)
			}
			end += dataSize
		}

		// Prefer the whole selected-alphabet token. This preserves bare CIDs
		// whose encoded data legitimately contains '/' or required '=' padding.
		if !hasCIDEmbeddingAfter(line, end) {
			if candidate, ok := decodeCIDCandidate(line[start:end]); ok {
				candidates = append(candidates, candidate)
				start = end
				continue
			}
		}

		// If the maximal token is not a CID, a delimiter within it may be the
		// actual boundary. Earlier '/' or '=' runes that are encoded data fail
		// cid.Decode because the CID's declared multihash length is incomplete.
		found := false
		for _, candidateEnd := range delimiterEnds {
			if candidate, ok := decodeCIDCandidate(line[start:candidateEnd]); ok {
				candidates = append(candidates, candidate)
				start = candidateEnd
				found = true
				break
			}
		}
		if found {
			continue
		}

		// A malformed token must not hide a valid candidate following a shared
		// structural delimiter. Resume at its first delimiter when one exists.
		if len(delimiterEnds) > 0 {
			start = delimiterEnds[0]
		} else if end > start {
			start = end
		} else {
			start += size
		}
	}
	return candidates
}

func cidTextEncodingAt(line string, start int, value rune, size int) (encoding multibase.Encoding, dataStart int, ok bool) {
	if strings.HasPrefix(line[start:], "Qm") {
		return multibase.Base58BTC, start, true
	}
	encoding = multibase.Encoding(value)
	if encoding == multibase.Identity {
		return 0, 0, false
	}
	if _, ok = multibase.EncodingToStr[encoding]; !ok {
		return 0, 0, false
	}
	return encoding, start + size, true
}

func isMultibaseDataRune(encoding multibase.Encoding, value rune) bool {
	switch encoding {
	case multibase.Base2:
		return value == '0' || value == '1'
	case multibase.Base16, multibase.Base16Upper:
		return isASCIIDigit(value) || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
	case multibase.Base32, multibase.Base32Upper:
		return isASCIIAlpha(value) || value >= '2' && value <= '7'
	case multibase.Base32pad, multibase.Base32padUpper:
		return isASCIIAlpha(value) || value >= '2' && value <= '7' || value == '='
	case multibase.Base32hex, multibase.Base32hexUpper:
		return isASCIIDigit(value) || value >= 'a' && value <= 'v' || value >= 'A' && value <= 'V'
	case multibase.Base32hexPad, multibase.Base32hexPadUpper:
		return isASCIIDigit(value) || value >= 'a' && value <= 'v' || value >= 'A' && value <= 'V' || value == '='
	case multibase.Base36, multibase.Base36Upper:
		return isASCIIAlpha(value) || isASCIIDigit(value)
	case multibase.Base58BTC:
		return strings.ContainsRune("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz", value)
	case multibase.Base58Flickr:
		return strings.ContainsRune("123456789abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ", value)
	case multibase.Base64:
		return isASCIIAlpha(value) || isASCIIDigit(value) || value == '+' || value == '/'
	case multibase.Base64url:
		return isASCIIAlpha(value) || isASCIIDigit(value) || value == '-' || value == '_'
	case multibase.Base64pad:
		return isASCIIAlpha(value) || isASCIIDigit(value) || value == '+' || value == '/' || value == '='
	case multibase.Base64urlPad:
		return isASCIIAlpha(value) || isASCIIDigit(value) || value == '-' || value == '_' || value == '='
	case multibase.Base256Emoji:
		// The actual base256emoji alphabet is a curated subset of Unicode
		// symbols. Ask the selected decoder instead of treating every symbol
		// (including the structural '=' delimiter) as encoded data.
		_, decoded, err := multibase.Decode(string(multibase.Base256Emoji) + string(value))
		return err == nil && len(decoded) == 1
	default:
		return false
	}
}

func isASCIIAlpha(value rune) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func isASCIIDigit(value rune) bool {
	return value >= '0' && value <= '9'
}

func hasCIDEmbeddingBefore(line string, start int) bool {
	if start == 0 {
		return false
	}
	value, _ := utf8.DecodeLastRuneInString(line[:start])
	return isCIDEmbeddingRune(value)
}

func hasCIDEmbeddingAfter(line string, end int) bool {
	if end == len(line) {
		return false
	}
	value, _ := utf8.DecodeRuneInString(line[end:])
	return isCIDEmbeddingRune(value)
}

func isCIDEmbeddingRune(value rune) bool {
	return unicode.IsLetter(value) || unicode.IsNumber(value) || unicode.IsMark(value) || value == '_'
}

func decodeCIDCandidate(candidate string) (decodedCIDCandidate, bool) {
	// Match isObjectIdShaped's cheap length gate before doing the real parse.
	if len(candidate) < 46 {
		return decodedCIDCandidate{}, false
	}
	decoded, err := cid.Decode(candidate)
	if err != nil || decoded.Version() != 0 && decoded.Version() != 1 {
		return decodedCIDCandidate{}, false
	}
	return decodedCIDCandidate{text: candidate, decoded: decoded}, true
}

func isRepeatedCIDSentinel(candidate string, decoded cid.Cid) bool {
	if decoded.Version() != 1 {
		return false
	}
	for _, prefix := range []string{"bafyrei", "bafybei", "bafkrei"} {
		if strings.HasPrefix(candidate, prefix) && hasRepeatedPayload(candidate, len(prefix)) {
			return true
		}
	}
	return false
}

func hasRepeatedPayload(value string, prefix int) bool {
	if prefix >= len(value) {
		return false
	}
	payload := value[prefix:]
	return strings.Count(payload, payload[:1]) == len(payload)
}

func hasAccountIdentityBoundaries(line string, start, end int) bool {
	return (start == 0 || !isBase58Byte(line[start-1])) &&
		(end == len(line) || !isBase58Byte(line[end]))
}

func isBase58Byte(value byte) bool {
	return value >= '1' && value <= '9' || value >= 'A' && value <= 'H' ||
		value >= 'J' && value <= 'N' || value >= 'P' && value <= 'Z' ||
		value >= 'a' && value <= 'k' || value >= 'm' && value <= 'z'
}

func hasAttachedSyntheticAccountSuffix(line string, accountEnd int) bool {
	const suffix = "#SYNTHETIC_member"
	if accountEnd < 0 || accountEnd > len(line) || !strings.HasPrefix(line[accountEnd:], suffix) {
		return false
	}
	end := accountEnd + len(suffix)
	if end == len(line) {
		return true
	}
	// The exemption is a complete token, not a prefix. Keep the terminator
	// grammar intentionally narrow so punctuation and Unicode cannot extend it.
	switch line[end] {
	case ' ', '\t', '\r', '\n', '"', '\'', '`', ',', ')', ']', '}':
		return true
	default:
		return false
	}
}

func isSafeContextualValue(candidate string) bool {
	return syntheticValueMarker.MatchString(candidate) || hasSingleRepeatedByte(candidate)
}

// isPublishableAddress admits the documentation domain and the project's own
// published role addresses. A role address is the contact a public repository
// is supposed to carry — it names a function, not a person, so it discloses no
// individual. Every other address is treated as corpus-derived: an employee's
// mailbox at the same domain is NOT exempt, which is why this is a literal
// allowlist rather than a domain suffix.
func isPublishableAddress(address string) bool {
	lower := strings.ToLower(address)
	if strings.HasSuffix(lower, "@example.com") {
		return true
	}
	for _, published := range publishedRoleAddresses {
		if lower == published {
			return true
		}
	}
	return false
}

// publishedRoleAddresses is the closed set this repository may carry, and is
// deliberately short. Adding to it is a decision to publish that address.
var publishedRoleAddresses = []string{
	"security@anytype.io",
}

func hasSingleRepeatedByte(value string) bool {
	return value != "" && strings.Count(value, value[:1]) == len(value)
}

func looksEncodedAndHighEntropy(value string) bool {
	if len(value) < 20 {
		return false
	}
	counts := map[byte]int{}
	hasDigitOrBase64Punctuation := false
	for i := range value {
		counts[value[i]]++
		if value[i] >= '0' && value[i] <= '9' || strings.ContainsRune("+/=", rune(value[i])) {
			hasDigitOrBase64Punctuation = true
		}
	}
	if len(counts) < 8 || !hasDigitOrBase64Punctuation && !isBase58Text(value) {
		return false
	}
	var entropy float64
	for _, count := range counts {
		probability := float64(count) / float64(len(value))
		entropy -= probability * math.Log2(probability)
	}
	return entropy >= 3.25
}

func isBase58Text(value string) bool {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for i := range value {
		if !strings.ContainsRune(alphabet, rune(value[i])) {
			return false
		}
	}
	return value != ""
}

func TestSemanticSecretScannerRejectsEveryShapeAcrossEveryTextSurface(t *testing.T) {
	base58Alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	account := "A" + base58Alphabet[:47]
	cidV1 := generateTestCID(t, cid.Prefix{
		Version: 1, Codec: cid.DagCBOR, MhType: mh.SHA2_256, MhLength: -1,
	}, "semantic scanner DAG-CBOR").String()
	rawCIDV1 := generateTestCID(t, cid.Prefix{
		Version: 1, Codec: cid.Raw, MhType: mh.SHA2_256, MhLength: -1,
	}, "semantic scanner raw").String()
	cidV0 := generateTestCID(t, cid.Prefix{
		Version: 0, Codec: cid.DagProtobuf, MhType: mh.SHA2_256, MhLength: -1,
	}, "semantic scanner CIDv0").String()
	generatedRetired := "GENERATED_TEST_ONLY_RETIRED_VALUE_0123456789"
	retired := []retiredCorpusSignature{newRetiredCorpusSignature(generatedRetired)}
	nearRetired := generatedRetired[:len(generatedRetired)-1] + differentLastByte(generatedRetired)

	mutations := map[string]string{
		"account identity":  account,
		"CIDv1":             cidV1,
		"raw CIDv1":         rawCIDV1,
		"CIDv0":             cidV0,
		"email":             "owner" + "@" + "private.invalid",
		"retired exact":     generatedRetired,
		"retired near":      nearRetired,
		"GitHub fine grain": "github_" + "pat_" + strings.Repeat("Ab1_", 7),
		"AWS session":       "AS" + "IA" + strings.Repeat("A1", 8),
		"npm":               "npm" + "_" + strings.Repeat("Ab1", 8),
		"Stripe live":       "sk_" + "live_" + strings.Repeat("Ab1", 7),
		"Google API":        "AI" + "za" + strings.Repeat("Ab1_", 7),
		"contextual base58": `"invite_key": "` + base58Alphabet[:40] + `"`,
		"contextual base64": `"file_key": "` + "QWxhZGRp" + "bjpvcGVuIHNlc2FtZTIzNDU2Nzg5Ky8=" + `"`,
	}
	paths := []string{
		"audit.md",
		"codec/anyblockjson/production.go",
		"format/v2/schema/production.json",
		"internal/testfixtures/helper.go",
		"codec/anyblockjson/mutation_test.go",
		"format/v2/examples/mutation.md",
		"format/v2/conformance/mutation.json",
	}

	for _, path := range paths {
		for name, mutation := range mutations {
			t.Run(path+"/"+name, func(t *testing.T) {
				findings := scanTextualSecretsWithRetiredSignatures(path, mutation, retired)
				require.NotEmptyf(t, findings, "%s must be rejected in every textual location", name)
			})
		}
	}
}

func differentLastByte(value string) string {
	if value[len(value)-1] == 'Z' {
		return "Y"
	}
	return "Z"
}

func TestSemanticSecretScannerAllowsOnlyDocumentedSafeShapes(t *testing.T) {
	account := "A" + strings.Repeat("1", 47)
	cidV1 := "bafyrei" + strings.Repeat("a", 52)
	malformedCIDV0 := "Qm" + strings.Repeat("1", 44)
	_, err := cid.Decode(malformedCIDV0)
	require.Error(t, err)
	tests := []struct {
		name    string
		path    string
		content string
	}{
		{"example email", "README.md", "contact new@example.com"},
		{"published role address", "README.md", "email security@anytype.io for findings"},
		{"repeated account sentinel", "format/v2/SPEC.md", account},
		{"repeated CIDv1 sentinel", "bundle/DESIGN.md", cidV1},
		{"malformed CIDv0 lookalike", "examples/object.md", malformedCIDV0},
		{"explicit synthetic marker", "fixture.go", `invite_key = "SYNTHETIC_INVITE_KEY_0000001"`},
		{"ordinary non-secret data", "format/v2/SPEC.md", "abcdefghijklmnopqrstuvwxyz0123456789"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Empty(t, scanTextualSecrets(tc.path, tc.content))
		})
	}
}

// The role-address exemption is a literal allowlist, not a domain rule: the
// project's published security contact is publishable, an individual's mailbox
// at the same domain is exactly the corpus leak the scan exists to catch.
func TestPublishedRoleAddressExemptionDoesNotWidenToItsDomain(t *testing.T) {
	assert.Empty(t, scanTextualSecrets("README.md", "email security@anytype.io"))
	// Assembled at run time: a literal here would be a non-example address in a
	// scanned file, and TestRepositoryFixtureSecretScan would flag this test.
	const domain = "@anytype" + ".io"
	for _, personal := range []string{
		"someone" + domain,
		"security" + domain + ".attacker.example",
		"notsecurity" + domain,
	} {
		assert.NotEmpty(t, scanTextualSecrets("README.md", "contact "+personal),
			"%q must not inherit the role address exemption", personal)
	}
}

func TestRetiredCorpusSignaturesAreNonReversibleAndKeepExactNearDetection(t *testing.T) {
	for _, signature := range retiredCorpusSignatures {
		require.Positive(t, signature.length)
		for _, digest := range []string{signature.exactSHA256, signature.leftSHA256, signature.rightSHA256} {
			require.Len(t, digest, sha256.Size*2)
			assert.True(t, hexHashPattern.MatchString(digest))
			assert.Empty(t, accountIdentityPattern.FindStringSubmatch(digest))
			assert.Empty(t, decodedCIDCandidates(digest))
		}
	}

	generated := []string{
		"GENERATED_TEST_ONLY_INVITE_VALUE_0123456789",
		"GENERATED_TEST_ONLY_FILE_VALUE_ABCDEFGHIJ0123456789",
	}
	signatures := make([]retiredCorpusSignature, 0, len(generated))
	for _, value := range generated {
		signatures = append(signatures, newRetiredCorpusSignature(value))
	}
	for _, value := range generated {
		exact, near := retiredMatchesOnLine("prefix="+value+";suffix", signatures)
		assert.True(t, exact)
		assert.False(t, near)

		mutated := value[:len(value)/2] + differentLastByte(value) + value[len(value)/2+1:]
		exact, near = retiredMatchesOnLine(mutated, signatures)
		assert.False(t, exact)
		assert.True(t, near)
	}
}

func TestSyntheticExemptionsAreCandidateScoped(t *testing.T) {
	base58Alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	account := "A" + base58Alphabet[:47]
	secondAccount := "A" + base58Alphabet[1:48]
	repeatedAccount := "A" + strings.Repeat("1", 47)
	credential := base58Alphabet[:40]

	rejected := map[string]string{
		"unrelated prose":              account + " // unrelated SYNTHETIC prose",
		"substring lookalike":          account + " // NONSYNTHETIC",
		"neighboring account field":    `{"owner":"` + account + `","note":"SYNTHETIC"}`,
		"neighboring credential field": `{"invite_key":"` + credential + `","note":"SYNTHETIC"}`,
		"neighboring Go field":         `InviteKey: "` + credential + `", Note: "SYNTHETIC"`,
		"value marker prefix":          `invite_key = "NONSYNTHETIC_` + credential + `"`,
		"value marker suffix":          `invite_key = "SYNTHETICITY_` + credential + `"`,
		"account suffix prefix":        account + "#SYNTHETICITY_member",
		"account suffix extension":     account + "#SYNTHETIC_member_extra",
		"account suffix hyphen":        account + "#SYNTHETIC_member-extra",
		"account suffix dot":           account + "#SYNTHETIC_member.extra",
		"account suffix Unicode":       account + "#SYNTHETIC_memberé",
		"authorized plus unrelated":    repeatedAccount + "#SYNTHETIC_member " + account,
	}
	for name, content := range rejected {
		t.Run("reject/"+name, func(t *testing.T) {
			require.NotEmpty(t, scanTextualSecrets("ordinary.md", content))
		})
	}

	for name, content := range map[string]string{
		"sentinel then real/space":  repeatedAccount + " " + account,
		"sentinel then real/comma":  repeatedAccount + "," + account,
		"sentinel then real/slash":  repeatedAccount + "/" + account,
		"sentinel then real/equals": repeatedAccount + "=" + account,
		"real then sentinel/space":  account + " " + repeatedAccount,
		"real then sentinel/comma":  account + "," + repeatedAccount,
		"real then sentinel/slash":  account + "/" + repeatedAccount,
		"real then sentinel/equals": account + "=" + repeatedAccount,
	} {
		t.Run("adjacent/"+name, func(t *testing.T) {
			findings := scanTextualSecrets("ordinary.md", content)
			assert.Equal(t, 1, countFindingKind(findings, "non-synthetic 48-character account identity"))
		})
	}
	for name, content := range map[string]string{
		"two real/space":  account + " " + secondAccount,
		"two real/comma":  account + "," + secondAccount,
		"two real/slash":  account + "/" + secondAccount,
		"two real/equals": account + "=" + secondAccount,
	} {
		t.Run("adjacent/"+name, func(t *testing.T) {
			findings := scanTextualSecrets("ordinary.md", content)
			assert.Equal(t, 2, countFindingKind(findings, "non-synthetic 48-character account identity"))
		})
	}

	accepted := map[string]string{
		"attached account suffix at EOF": account + "#SYNTHETIC_member",
		"attached suffix closing quote":  `"` + account + `#SYNTHETIC_member"`,
		"attached suffix JSON comma":     `{"owner":"` + account + `#SYNTHETIC_member","next":true}`,
		"attached suffix array close":    `[` + account + `#SYNTHETIC_member]`,
		"attached suffix whitespace":     account + "#SYNTHETIC_member next",
		"repeated account":               repeatedAccount,
		"marker inside value":            `invite_key = "SYNTHETIC_` + credential + `"`,
	}
	for name, content := range accepted {
		t.Run("accept/"+name, func(t *testing.T) {
			assert.Empty(t, scanTextualSecrets("ordinary.md", content))
		})
	}
}

func TestCIDScannerCoversAcceptedFamiliesAndExactBoundaries(t *testing.T) {
	canonical := map[string]cid.Cid{
		"DAG-CBOR CIDv1": generateTestCID(t, cid.Prefix{
			Version: 1, Codec: cid.DagCBOR, MhType: mh.SHA2_256, MhLength: -1,
		}, "canonical DAG-CBOR"),
		"DAG-PB CIDv1": generateTestCID(t, cid.Prefix{
			Version: 1, Codec: cid.DagProtobuf, MhType: mh.SHA2_256, MhLength: -1,
		}, "canonical DAG-PB"),
		"raw CIDv1": generateTestCID(t, cid.Prefix{
			Version: 1, Codec: cid.Raw, MhType: mh.SHA2_256, MhLength: -1,
		}, "canonical raw"),
		"CIDv0": generateTestCID(t, cid.Prefix{
			Version: 0, Codec: cid.DagProtobuf, MhType: mh.SHA2_256, MhLength: -1,
		}, "canonical CIDv0"),
	}
	paths := []string{
		"ordinary.md",
		"codec/anyblockjson/production.go",
		"format/v2/schema/production.json",
		"internal/testfixtures/helper.go",
		"codec/anyblockjson/mutation_test.go",
		"format/v2/examples/object.md",
		"format/v2/conformance/object.json",
	}
	for _, path := range paths {
		for name, generated := range canonical {
			t.Run(path+"/"+name, func(t *testing.T) {
				encoded := generated.String()
				requireCIDRoundTrip(t, encoded, generated.Version())
				findings := scanTextualSecrets(path, encoded)
				require.Len(t, findings, 1)
				assert.Equal(t, fmt.Sprintf("non-synthetic CIDv%d content address", generated.Version()), findings[0].Kind)
			})
		}
	}

	sha512 := generateTestCID(t, cid.Prefix{
		Version: 1, Codec: cid.DagCBOR, MhType: mh.SHA2_512, MhLength: -1,
	}, "alternative sha2-512")
	sha3 := generateTestCID(t, cid.Prefix{
		Version: 1, Codec: cid.DagProtobuf, MhType: mh.SHA3_256, MhLength: -1,
	}, "alternative sha3-256")
	identity := generateTestCID(t, cid.Prefix{
		Version: 1, Codec: cid.Raw, MhType: mh.IDENTITY, MhLength: -1,
	}, strings.Repeat("identity payload ", 4))
	raw := canonical["raw CIDv1"]
	alternatives := map[string]string{
		"sha2-512 base32":  sha512.String(),
		"sha3-256 base32":  sha3.String(),
		"identity base32":  identity.String(),
		"base58btc":        cidInBase(t, raw, multibase.Base58BTC),
		"base64":           cidInBase(t, raw, multibase.Base64),
		"base64url padded": cidInBase(t, raw, multibase.Base64urlPad),
		"base256emoji":     cidInBase(t, raw, multibase.Base256Emoji),
	}
	for name, encoded := range alternatives {
		t.Run("alternative/"+name, func(t *testing.T) {
			requireCIDRoundTrip(t, encoded, 1)
			findings := scanTextualSecrets("ordinary.md", encoded)
			require.Len(t, findings, 1)
			assert.Equal(t, "non-synthetic CIDv1 content address", findings[0].Kind)
		})
	}
	for encoding, name := range multibase.EncodingToStr {
		if encoding == multibase.Identity {
			continue // NUL-prefixed identity encoding is a binary, not textual, surface.
		}
		t.Run("multibase/"+name, func(t *testing.T) {
			encoded := cidInBase(t, raw, encoding)
			requireCIDRoundTrip(t, encoded, 1)
			findings := scanTextualSecrets("ordinary.md", encoded)
			require.Len(t, findings, 1)
			assert.Equal(t, "non-synthetic CIDv1 content address", findings[0].Kind)
		})
	}

	real := raw.String()
	for _, prefix := range []string{"bafyrei", "bafybei", "bafkrei"} {
		sentinel := prefix + strings.Repeat("a", 52)
		requireCIDRoundTrip(t, sentinel, 1)
		assert.Empty(t, scanTextualSecrets("ordinary.md", sentinel),
			"documented repeated CIDv1 sentinel")
	}

	sentinel := "bafkrei" + strings.Repeat("a", 52)
	for name, content := range map[string]string{
		"comma":      sentinel + "," + real,
		"whitespace": sentinel + " " + real,
		"slash":      sentinel + "/" + real,
		"equals":     sentinel + "=" + real,
		"JSON":       `{"safe":"` + sentinel + `","real":"` + real + `"}`,
	} {
		t.Run("sentinel then real/"+name, func(t *testing.T) {
			findings := scanTextualSecrets("ordinary.md", content)
			require.Len(t, findings, 1)
			assert.Equal(t, "non-synthetic CIDv1 content address", findings[0].Kind)
		})
	}

	for name, content := range map[string]string{
		"comma separated":  real + "," + sha512.String(),
		"slash separated":  real + "/" + sha512.String(),
		"equals separated": real + "=" + sha512.String(),
		"quoted":           `"` + real + `" "` + sha3.String() + `"`,
		"punctuation":      "(" + real + ")." + sha512.String() + "#object",
	} {
		t.Run("multiple/"+name, func(t *testing.T) {
			findings := scanTextualSecrets("ordinary.md", content)
			require.Len(t, findings, 2)
			for _, finding := range findings {
				assert.Equal(t, "non-synthetic CIDv1 content address", finding.Kind)
			}
		})
	}

	for name, content := range map[string]string{
		"assignment":    "object_id=" + real,
		"path":          "/objects/" + real + "/asset",
		"query":         "/object?object_id=" + real + "&view=full",
		"slash suffix":  real + "/asset",
		"equals suffix": real + "=value",
		"slash prefix":  "asset/" + real,
		"equals prefix": "object_id=" + real,
	} {
		t.Run("boundary/"+name, func(t *testing.T) {
			findings := scanTextualSecrets("ordinary.md", content)
			require.Len(t, findings, 1)
			assert.Equal(t, "non-synthetic CIDv1 content address", findings[0].Kind)
		})
	}

	base58Alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	malformedQm := "Qm" + strings.Repeat(base58Alphabet, 2)[:44]
	_, err := cid.Decode(malformedQm)
	require.Error(t, err)
	malformed := map[string]string{
		"invalid Qm multihash": malformedQm,
		"alphanumeric prefix":  "0" + real,
		"alphabetic prefix":    "x" + real,
		"alphabetic suffix":    real + "x",
		"Unicode prefix":       "é" + real,
		"truncated":            real[:len(real)-1],
		"invalid base32 byte":  real[:len(real)-1] + "1",
	}
	for name, candidate := range malformed {
		t.Run("malformed/"+name, func(t *testing.T) {
			_, decodeErr := cid.Decode(candidate)
			require.Error(t, decodeErr)
			assert.Empty(t, scanTextualSecrets("ordinary.md", candidate))
		})
	}
}

func TestCIDScannerCoversEveryTextualMultibaseDelimiterShape(t *testing.T) {
	first := generateTestCID(t, cid.Prefix{
		Version: 1, Codec: cid.Raw, MhType: mh.SHA2_256, MhLength: -1,
	}, "all-base delimiter first")
	second := generateTestCID(t, cid.Prefix{
		Version: 1, Codec: cid.DagCBOR, MhType: mh.SHA2_512, MhLength: -1,
	}, "all-base delimiter second")

	for encoding, name := range multibase.EncodingToStr {
		if encoding == multibase.Identity {
			continue // NUL-prefixed identity encoding is a binary, not textual, surface.
		}
		a := cidInBase(t, first, encoding)
		b := cidInBase(t, second, encoding)
		requireCIDRoundTrip(t, a, 1)
		requireCIDRoundTrip(t, b, 1)

		shapes := map[string]struct {
			content string
			want    int
		}{
			"assignment prefix": {content: "object_id=" + a, want: 1},
			"path suffix":       {content: a + "/asset", want: 1},
			"query":             {content: "/object?object_id=" + a + "&view=full", want: 1},
			"equals suffix":     {content: a + "=value", want: 1},
			"two real slash":    {content: a + "/" + b, want: 2},
			"two real equals":   {content: a + "=" + b, want: 2},
		}
		for shape, test := range shapes {
			t.Run(name+"/"+shape, func(t *testing.T) {
				findings := scanTextualSecrets("ordinary.md", test.content)
				assert.Equal(t, test.want,
					countFindingKind(findings, "non-synthetic CIDv1 content address"))
			})
		}
	}
}

func generateTestCID(t testing.TB, prefix cid.Prefix, seed string) cid.Cid {
	t.Helper()
	generated, err := prefix.Sum([]byte(seed))
	require.NoError(t, err)
	requireCIDRoundTrip(t, generated.String(), prefix.Version)
	return generated
}

func cidInBase(t testing.TB, generated cid.Cid, base multibase.Encoding) string {
	t.Helper()
	encoded, err := generated.StringOfBase(base)
	require.NoError(t, err)
	return encoded
}

func requireCIDRoundTrip(t testing.TB, encoded string, version uint64) {
	t.Helper()
	require.GreaterOrEqual(t, len(encoded), 46)
	decoded, err := cid.Decode(encoded)
	require.NoError(t, err)
	require.Equal(t, version, decoded.Version())
}

func TestContextualEntropyCoversQuotedAndUnquotedAssignments(t *testing.T) {
	base58 := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"[:40]
	digitlessBase58 := "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	base64 := "QWxhZGRpbjpvcGVuIHNlc2FtZTIzNDU2Nzg5Ky8="
	forms := map[string]func(string) string{
		"JSON quoted":         func(value string) string { return `{"invite_key":"` + value + `"}` },
		"JSON unquoted":       func(value string) string { return `{"invite_key":` + value + `}` },
		"Go quoted":           func(value string) string { return `inviteKey := "` + value + `"` },
		"Go unquoted":         func(value string) string { return `inviteKey := ` + value },
		"YAML quoted":         func(value string) string { return `invite_key: "` + value + `"` },
		"YAML unquoted":       func(value string) string { return "invite_key: " + value },
		"TOML quoted":         func(value string) string { return `password = "` + value + `"` },
		"TOML triple basic":   func(value string) string { return `password = """` + value + `"""` },
		"TOML triple literal": func(value string) string { return "password = '''" + value + "'''" },
		"TOML unquoted":       func(value string) string { return "password = " + value },
		"dotenv quoted":       func(value string) string { return `API_KEY="` + value + `"` },
		"dotenv unquoted":     func(value string) string { return "API_KEY=" + value },
		"shell quoted":        func(value string) string { return `export ACCESS_TOKEN="` + value + `"` },
		"shell ANSI-C":        func(value string) string { return `export ACCESS_TOKEN=$'` + value + `'` },
		"shell unquoted":      func(value string) string { return "export ACCESS_TOKEN=" + value },
	}
	for _, value := range []string{base58, digitlessBase58, base64} {
		for name, form := range forms {
			t.Run(name+"/"+value[:4], func(t *testing.T) {
				content := form(value)
				matches := sensitiveAssignmentPattern.FindAllStringSubmatch(content, -1)
				require.Len(t, matches, 1)
				require.Len(t, matches[0], 2)
				assert.Equal(t, value, matches[0][1], "the capture contains only the exact assigned value")
				findings := scanTextualSecrets("ordinary.txt", content)
				require.NotEmpty(t, findings)
				assert.Equal(t, "high-entropy value in a credential/key context", findings[len(findings)-1].Kind)
			})
		}
	}

	clean := []string{
		"type FileEncryptionKey string",
		`const inviteKeyType = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefg"`,
		"ordinaryIdentifier123456789ABCDEFGHJKLMNPQRSTUVWXYZ",
		`const ordinaryBase58 = "` + digitlessBase58 + `"`,
		"homepage: https://example.com/abcdefghijklmnopqrstuvwxyz0123456789",
		"sha1: 0123456789abcdef0123456789abcdef01234567",
		"sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"invite_key: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"password: correcthorsebatterystaple",
	}
	for _, content := range clean {
		assert.Empty(t, scanTextualSecrets("ordinary.txt", content), content)
	}
}

func TestSemanticSecretScannerClassifiesBinaryDeliberately(t *testing.T) {
	assert.True(t, isTextualSource([]byte("plain UTF-8: 世界\n")))
	assert.False(t, isTextualSource([]byte{'t', 'e', 'x', 't', 0, 'x'}), "NUL-bearing bytes are binary")
	assert.False(t, isTextualSource([]byte{0xff, 0xfe, 0xfd}), "invalid UTF-8 is binary")
}

func TestDocumentedSyntheticIdentitySentinelsPreserveShapesAndReferences(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	read := func(relative string) string {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		require.NoError(t, err)
		return string(data)
	}

	account := "A" + strings.Repeat("1", 47)
	objectCID := "bafyrei" + strings.Repeat("a", 52)
	typeCID := "bafyrei" + strings.Repeat("b", 52)
	fileCID := "bafyrei" + strings.Repeat("c", 52)
	require.Len(t, account, 48)
	for _, value := range []string{objectCID, typeCID, fileCID} {
		require.Len(t, value, 59)
		require.True(t, hasRepeatedPayload(value, 7))
	}

	spec := read("format/v2/SPEC.md")
	assert.Equal(t, 2, strings.Count(spec, account+"#SYNTHETIC_member"),
		"creator and modifier retain the same synthetic participant reference")

	design := read("bundle/DESIGN.md")
	assert.Equal(t, 1, strings.Count(design, objectCID+".anyblock.json"))
	assert.Equal(t, 1, strings.Count(design, typeCID+".anyblock.json"))
	assert.Equal(t, 2, strings.Count(design, fileCID+".anyblock.json"),
		"the naming example and adjacency document reuse the same file-object id")
	assert.Equal(t, 1, strings.Count(design, fileCID+".png"),
		"the adjacent blob keeps the document's exact synthetic id")
	assert.Equal(t, 1, strings.Count(design, account+".anyblock.json"))
}
