# AnyBlock inline markup

Every text-bearing block in an AnyBlock v2 document carries one `text` string,
with the formatting written inline. There are no offsets anywhere in the
format.

```json
{ "type": "paragraph", "text": "Ship the **new export** with <mention object_id=\"bafyreialice\">Alice</mention>" }
```

It looks like Markdown, and the resemblance is deliberate — but **it is not
Markdown**, and handing a `text` string to a CommonMark library will get some
of it wrong, some of it silently. This page is for the person writing the
parser. `SPEC.md` §8 is normative; this is the same rules stated as what to do
and what will bite you. Every example below was produced by running the string
through this repository's parser and re-rendering it, and the whole table is
pinned by a test.

---

## The complete grammar

The first useful fact is that this list is all of it.

| Syntax | Mark | Notes |
|---|---|---|
| `**text**` | bold | |
| `*text*` | italic | canonical; `_text_` accepted on input, re-exported as `*text*` |
| `~~text~~` | strikethrough | |
| `` `text` `` | inline code | CommonMark code-span rules; content is literal |
| `[text](url)` | link | |
| `[text](anytype://object?objectId=<id>)` | object link | the **exact** one-parameter form only |
| `<mention object_id="<id>">text</mention>` | mention | a decorated object reference |
| `<u>text</u>` | underline | |
| `<font color="red">text</font>` | text colour | Anytype colour names |
| `<font background="yellow">text</font>` | background colour | a coincident pair combines: `<font color="red" background="yellow">` |

Plus `\n`, a soft line break inside the block. **Everything else in the string
is literal text.**

There is one mark you cannot write: an emoji mark is materialized into the text
on export — the emoji replaces the covered characters. It is the format's only
deliberate loss.

---

## What a CommonMark parser gets wrong

Every row was run through this repository's parser: the first column is the
string, the second is what comes out, the third is the rule it demonstrates.
Where canonical output rewrites the string, the row says so.

### Constructs CommonMark has and this dialect does not

| you write | you get | |
|---|---|---|
| `![alt](img.png)` | text `!alt`, with a **link** mark on `alt` | there are no inline images. The `!` is left as a character and the rest parses as a link — the single most likely silent mis-render |
| `<https://example.com>` | literal text | no autolinks |
| `see https://example.com` | literal text | no bare-URL linkification either (that is GFM, not CommonMark, and not this) |
| `[t][ref]` | literal text | no reference links, no link reference definitions |
| `# not a heading` | literal text | **no block syntax at all** inside `text`. Block structure is the `blocks` array |
| `- not a list` | literal text | |
| `> not a quote` | literal text | |
| `\| a \| b \|` | literal text | |
| `---` | literal text | |
| `<b>x</b>`, `<sub>x</sub>` | literal text | no raw HTML beyond the three tags above |
| `<U>u</U>` | literal text | **tag names are case-sensitive**; `<u>` is a mark, `<U>` is prose |
| `trailing two spaces  \n` | kept verbatim | two trailing spaces are not a hard break; `\n` is already the soft break |

### Constructs this dialect has and CommonMark does not

| you write | you get |
|---|---|
| `<u>u</u>` | an underline mark — not raw HTML to pass through |
| `<font color="red">r</font>` | a text-colour mark |
| `<font background="yellow" color="red">rb</font>` | both marks; canonical output re-orders to `color` then `background` |
| `<font color='red'>r</font>` | accepted; canonical output uses double quotes |
| `<mention object_id="bafyreialice">A</mention>` | a mention of that object |
| `[t](anytype://object?objectId=bafyreialice)` | an **object** link — a reference into the space |
| `~~x~~` | strikethrough (GFM, not CommonMark core) |

### Where it parses the same syntax differently

| you write | you get | |
|---|---|---|
| `_x_` | italic | re-exported as `*x*`; canonical output never uses `_` |
| `__x__` | bold | |
| `a_b_c` | literal | intraword underscores stay literal, as in CommonMark |
| `==x==` | literal | |
| `~x~` | literal | only runs of exactly two tildes delimit |
| `^x^` | literal | |
| `&lt;u&gt;` | the text `<u>` | entities are decoded on input; canonical output writes `\<u>` instead |
| `[t](anytype://object?objectId=X&spaceId=Y)` | an **ordinary link**, param verbatim | a second parameter makes it not-an-object-link. Guessing which half is the id and guessing wrong is unrecoverable, so nothing guesses |

The delimiter set — `**`, `*`, `~~`, `` ` ``, `[…](…)` — is **closed**. A future
version adds a mark as a tag, never as new punctuation, which is what makes
`==x==` and friends safe to leave literal forever.

The parser is a deterministic delimiter stack, not CommonMark's delimiter-run
algorithm: the rule of three is not invertible, and this grammar has to be an
exact inverse of the renderer. For well-formed input the two agree; unmatched
Markdown delimiters demote to literal text either way.

---

## Three inputs that are errors, not text

Once `<u`, `<font` or `<mention` is recognised, strictness begins. A malformed
whitelisted tag is a **validation error**, so an agent gets a real message
instead of a silently dropped mark:

| you write | you get |
|---|---|
| `<u>unclosed` | `unclosed <u> tag` |
| `<font color="red">x</u>` | `misnested tags: </u> closes across <font>` |
| `<font size="3">x</font>` | `unknown attribute "size" on <font> tag` |

A zero-length tag — `<mention object_id="x"></mention>` — is dropped rather
than refused.

If you are writing a *renderer* rather than an importer you may reasonably
choose to degrade instead of failing. If you are writing something that feeds
documents back into Anytype, do not: these three are how a wrong document tells
you it is wrong.

---

## Escaping, and the space reserved for later

CommonMark backslash escapes apply: `\*`, `` \` ``, `\[`, `\]`, `\~`, `\<`,
`\\`. Input additionally accepts HTML entities. Canonical output uses
backslashes only, applied minimally — with one deliberate exception.

**Canonical output escapes `<` before any tag-shaped run**: `<`, an optional
`/`, then an ASCII letter. So prose containing `<sub>x</sub>` is written
`\<sub>x\</sub>`, even though `sub` is not a tag this version knows.

The reason matters for anyone storing these strings. A `text` string carries no
version marker. If `<` were escaped only before `u`, `font` and `mention`, then
a 2.0 document could hold a literal `<sub>x</sub>`, and the day a later version
defines `sub` those same bytes would read as markup — and because a malformed
instance of a *known* tag is an error, a stored document that was valid could
become invalid. Escaping the tag *shape* closes that: in canonical output an
unescaped `<` is never followed by a letter, and the whole `</?[A-Za-z]…>` space
is free for a later version with no migration.

Two consequences for your parser:

- Treat `\<` as literal `<`, always.
- Do not assume a `<` you see is a tag. Check the name.

Code spans follow CommonMark exactly, including that **backslash escapes do not
apply inside them**. A tag inside a code span is text: `` `<u>x</u>` `` renders
as the characters. This is why a parser that strips tags with a regex before
handling code spans is wrong — and in the audited corpus it is wrong on real
documents, where `` `<a href>` `` and `` `anytype://object?objectId=<ID>` ``
sit inside code spans in ordinary prose.

Link destinations render bare with `` \ ( ) & < [ ] ` `` backslash-escaped, or
angle-wrapped when the URL contains whitespace. Destinations longer than 2,048
UTF-16 code units, destinations surrounded by more than 32 whitespace
characters, and link labels nested more than 32 deep are **not recognised** —
the `[` stays literal. Those bounds keep parsing linear on untrusted input.

---

## Blocks whose text is never parsed

`code` and `embed` blocks carry raw content. Their `text` is verbatim — only
JSON string escaping applies — and any stored marks on them are dropped on
export. Skip inline parsing for these two types entirely.

In the audited space (the **Community** export, 3,286 documents, 16,228
text-bearing blocks), 55 of those blocks are `code` or `embed`. Four of the
seven occurrences of the `anytype://object?objectId=` deep-link string in that
export's block text are inside `code` blocks, where they are not links; a fifth
sits inside a code span; only two are object links.

---

## If you convert back to ranges

Internal marks are ranges over **UTF-16 code units**, not bytes and not runes.
Each maximal contiguous run of one mark is one range. If you round-trip through
ranges, expect normalization rather than byte-equality: overlapping ranges of
the same type resolve earlier-start-wins, emphasis delimiters shrink past
whitespace at their boundaries, and adjacent same-type ranges merge. §8.3 has
the exact order.

---

## What actually occurs

Re-derived from the **Community** export, 3,286 documents. Of its 16,228
text-bearing blocks, 55 are `code` or `embed`; the other 16,173 were run
through this repository's parser. **Zero parse errors, and all 16,173
re-rendered byte-identically** — which is the round-trip contract holding on
real data rather than on fixtures.

The marks those blocks carry:

| mark | count |
|---|---|
| bold | 1,179 |
| inline code | 188 |
| link | 181 |
| italic | 79 |
| text colour | 44 |
| background colour | 43 |
| underline | 35 |
| mention | 13 |
| object link | 2 |

The 44 + 43 colour marks come from 67 `<font>` tags: 24 colour only, 23
background only, 20 carrying both. Escapes in the same blocks: 249 `\_`, 11
`\[`, 3 `\*`, and **no** `\<` at all — no author in this space wrote prose that
looked like a tag. Forty blocks contain a soft line break (59 breaks).

So most text is plain, and the whole dialect is a small tail — which is exactly
why it is worth getting right rather than guessing: a parser that mishandles it
will look correct on almost every block it sees.

---

## A parser checklist

1. Skip `code` and `embed` block text entirely.
2. Handle backslash escapes first; `\<` is never a tag.
3. Handle code spans before tags: their content is literal.
4. Recognise `<u>`, `<font>`, `<mention>` by exact lowercase name; anything
   else `<…>` is text.
5. Reject a `font` attribute that is not `color` or `background`.
6. Treat `[t](anytype://object?objectId=X)` as an object reference only in its
   exact one-parameter form.
7. Do not linkify bare URLs, do not accept `<autolinks>`, do not accept images,
   do not look for block syntax.
8. Emphasis, strikethrough and code spans behave as you expect; leave them to
   your Markdown knowledge.

[`examples/reader/markup.go`](examples/reader/markup.go) implements 1–7 in one
standard-library Go file, as a display flattener: it renders to plain text and
does **not** raise the three errors above. That is the right trade for reading
and the wrong one for importing.
