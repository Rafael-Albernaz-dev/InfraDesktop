import { writeFileSync } from 'node:fs';
// npm packages may ship unrelated Go samples; keep them outside the product Go module.
writeFileSync(new URL('../frontend/node_modules/go.mod', import.meta.url), 'module infradesktop-npm-samples\n\ngo 1.27.3\n');
