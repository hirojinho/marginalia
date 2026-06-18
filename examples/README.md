# Examples

## `sample-corpus/`

A tiny, self-contained markdown corpus you can use to try marginalia in a few
minutes (see the project README's "Running locally" section). The documents are
original content written for this repository and are covered by the project's
[MIT license](../LICENSE) — free to copy, modify, and redistribute.

To use it, copy the markdown files into your vault's corpus directory:

```bash
mkdir -p "$VAULT_ROOT/data/corpus"
cp examples/sample-corpus/*.md "$VAULT_ROOT/data/corpus/"
```

marginalia indexes `$VAULT_ROOT/data/corpus/*.md` on startup, so the next launch
embeds these files and they become answerable through the chat/RAG tools.
