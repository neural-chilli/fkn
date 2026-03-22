# MCP Status

This package is intentionally disabled at the product level for now.

Why:

- `fkn` is currently optimized for direct CLI use, generated repo docs, and agents that can read `fkn.yaml` and run `fkn` commands directly.
- Shipping a public MCP surface creates a support and compatibility burden that is not part of the core product story right now.
- The code remains here so the design can be revisited later without re-learning everything from scratch.

Current policy:

- do not document MCP as a public `fkn` feature
- do not expose MCP through the CLI
- treat this package as internal and dormant unless the product direction changes
