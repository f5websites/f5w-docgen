# Alpha kitchen sink

A sample doc exercising every block, inline span, and link class the emitter renders, so the golden locks the whole template surface at once.

## Section one

This paragraph carries **bold**, *italic*, ~~struck~~, and `code`, plus a [VERIFY: confirm the number] marker.

The links paragraph: the [beta doc](../frameworks/beta.md), a scroll to [section two](#section-two), a drill into [node one](#node-one), a [frozen snapshot](../raw/snap.md), the [API contract](../frameworks/contract.yaml), an [external site](https://example.com/x), and the [site config](../docsite.json).

> [!NOTE]
> An informational callout.

> [!SECURITY]
> A security aside, shown or hidden by the lens.

> A plain blockquote, not a callout.

### Sub A

A subsection so the rail nests an H3 under its H2.

- a bullet
- another bullet with a nested list:
  - a nested item

An ordinary ordered list:

1. first plain item
2. second plain item

A steps list, where every item leads with a bold run:

1. **First.** the bold lead makes this a step.
2. **Second.** and this is the next step.

A task list:

- [ ] an undone task
- [x] a done task

```go
func main() {}
```

```diagram
+------+
| box  |
+------+
```

## Section two

A table with per-column alignment:

| Left | Center | Right |
| :--- | :---: | ---: |
| a | b | c |

## Related

- [Beta doc](../frameworks/beta.md) - the companion sample doc.
- A plain related bullet that is not a link.

## Details

### Node one
Source: SR-1, 2026-07-07.

The referenced node, which nests into [node two](#node-two).

### Node two

The nested node body, reached only from node one.

### Orphan node
Source: orphan provenance.

An unreferenced node, rendered as a visible section so its content is never lost.
