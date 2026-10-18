# README Diagrams

The repository README embeds rendered PNG diagrams so GitHub and VS Code display the same layout. The original Mermaid sources remain editable here. The dark background, three horizontal bands, agent ownership and authority gates are preserved. Images are rendered at twice the viewport resolution for readable text on high-density displays.

- [Architecture source](architecture.mmd) and [rendered architecture](architecture.png).
- [Three-proof logic source](logic-flow.mmd) and [rendered logic flow](logic-flow.png).

To edit a diagram, change its `.mmd` source and regenerate both images from the repository root:

```bash
node docs/diagrams/render.mjs
```

This optional authoring command needs Node.js, npm and the Mermaid CLI's headless browser. It pins Mermaid CLI 11.16.0 and Mermaid 11.17.2; npm may download those tools on first use. Running the Agentic Salmon demo does not require this documentation toolchain. Review the resulting images before committing them; regeneration on a different font/browser environment may change pixels even with the same Mermaid versions.

Check that sources, renderer and images match the recorded checksums without downloading tools:

```bash
node docs/diagrams/render.mjs --check
```

CI performs the equivalent checksum check with `sha256sum --check docs/diagrams/SHA256SUMS`. A source or renderer edit therefore requires regenerated images and updated checksums in the same commit. The checksums detect stale or modified artifacts; visual review establishes layout quality. The committed images are the publication artifacts, so readers do not depend on the Markdown host's Mermaid version or HTML-label handling.
