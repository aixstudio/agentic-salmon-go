// Optional documentation tooling; not needed to run the demo.
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const directory = dirname(fileURLToPath(import.meta.url));
const root = resolve(directory, '../..');
const names = ['architecture', 'logic-flow'];
const files = [...names.flatMap(name => [`${name}.mmd`, `${name}.png`]), 'render.mjs'];
const checksumFile = join(directory, 'SHA256SUMS');
const digest = name => createHash('sha256').update(readFileSync(join(directory, name))).digest('hex');
const checksums = () => files.map(name => `${digest(name)}  docs/diagrams/${name}\n`).join('');

if (process.argv.slice(2).some(arg => arg !== '--check')) {
  throw new Error('Usage: node docs/diagrams/render.mjs [--check]');
}

if (process.argv.includes('--check')) {
  if (readFileSync(checksumFile, 'utf8') !== checksums()) {
    throw new Error('Diagram sources, renderer or images changed. Regenerate the diagrams.');
  }
  console.log('Diagram sources, renderer and rendered images match recorded SHA-256 checksums.');
} else {
  for (const name of names) {
    const result = spawnSync('npx', [
      '--yes', '--package', '@mermaid-js/mermaid-cli@11.16.0',
      '--package', 'mermaid@11.17.2', 'mmdc',
      '-i', join(directory, `${name}.mmd`),
      '-o', join(directory, `${name}.png`),
      '-b', '#0D1117', '-w', '1800', '-s', '2',
    ], { cwd: root, stdio: 'inherit' });
    if (result.error) throw result.error;
    if (result.status !== 0) process.exit(result.status ?? 1);
  }
  writeFileSync(checksumFile, checksums());
  console.log('Rendered diagrams and refreshed SHA-256 checksums.');
}
