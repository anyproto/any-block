# File metadata without embedded bytes

This synthetic bundle contains one file object. Its `file_remote` value is
standard base64 of the JSON below. The payload has its own `version: 1` and
[schema](../../schema/file-remote.v1.schema.json); the containing document
still uses AnyBlock `formatVersion: "2.0"`.

```json
{
  "version": 1,
  "cid": "bafybeiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "encryption_keys": {
    "/0/": "SYNTHETIC_FILE_KEY"
  },
  "source_checksum": "synthetic-source-checksum",
  "variants": [
    {
      "cid": "bafybeiaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "path": "/0/",
      "checksum": "synthetic-variant-checksum",
      "mill": "/blob",
      "options": "",
      "width": 0
    }
  ]
}
```

`index.json.network_id` identifies the source network. There is no
`manifest.files` because this example contains no embedded bytes. A reader
uses the CID and exact path-key map to request remote file metadata and
content through its file service. The variant record is optional; a root
CID and an `encryption_keys` map are the minimum payload.

These CIDs, keys, and network identifier are synthetic fixtures. They show
the wire representation and are not intended for a live download. To inspect
the decoded payload using only Python's standard library:

```sh
python3 - <<'PYTHON'
import base64, json
from pathlib import Path
root = Path('format/v2/examples/remote_file')
doc = json.loads((root / 'files/file-demo.anyblock.json').read_text())
print(json.dumps(json.loads(base64.b64decode(doc['file_remote'])), indent=2))
PYTHON
```
