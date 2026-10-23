import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const guard = fileURLToPath(
  new URL("./architecture-frontend.mjs", import.meta.url),
);
function runGuard(files, prepare = () => {}) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "infradesktop-guard-"));
  try {
    for (const [name, code] of Object.entries(files)) {
      const target = path.join(root, "frontend/src", name);
      fs.mkdirSync(path.dirname(target), { recursive: true });
      fs.writeFileSync(target, code);
    }
    // Generated bindings are outside the authored tree, with only two approved imports.
    const bindings = path.join(root, "frontend/wailsjs/go/presentation");
    fs.mkdirSync(bindings, { recursive: true });
    fs.writeFileSync(
      path.join(bindings, "App.js"),
      "export const Status = () => {};",
    );
    fs.writeFileSync(
      path.join(bindings, "../models.ts"),
      "export namespace presentation {}",
    );
    prepare(root);
    const result = spawnSync(process.execPath, [guard], {
      cwd: root,
      encoding: "utf8",
    });
    assert.equal(result.error, undefined);
    return { status: result.status, output: result.stdout + result.stderr };
  } finally {
    fs.rmSync(root, { recursive: true, force: true });
  }
}

for (const [name, code] of [
  ["alias", "const bridge = window; bridge.go.presentation.App.Status()"],
  ["destructure", "const {go} = window; go.presentation.App.Status()"],
  [
    "renamed destructure",
    "const {go: bridge} = globalThis; bridge.presentation.App.Status()",
  ],
  [
    "computed alias",
    "const bridge = window; bridge['go'].presentation.App.Status()",
  ],
  ["implicit bridge", "go.presentation.App.Status()"],
  ["self alias", "const bridge = self; bridge.go.presentation.App.Status()"],
  [
    "document bridge",
    "const bridge = document.defaultView; bridge.go.presentation.App.Status()",
  ],
  ["vite glob", "const helpers = import.meta.glob('./*.js', { eager: true })"],
]) {
  test(`rejects ${name} without relying on property-chain spelling`, () => {
    const result = runGuard({ "view.ts": code });
    assert.equal(result.status, 1, result.output);
  });
}

for (const [name, files] of [
  [
    "explicit JavaScript helper",
    {
      "view.ts": 'import "./escape.js"',
      "escape.js": "window.go.presentation.App.Status()",
    },
  ],
  [
    "extensionless JavaScript helper",
    {
      "view.ts": 'import "./escape"',
      "escape.js": "window.go.presentation.App.Status()",
    },
  ],
  ["missing helper", { "view.ts": 'import "./missing"' }],
  [
    "transitive TypeScript alias",
    {
      "view.ts": 'import "./helper"',
      "helper.ts": "const bridge = window; bridge.go.presentation.App.Status()",
    },
  ],
  [
    "nested presentation impersonation",
    {
      "nested/presentation.ts":
        "import {Status} from '../wailsjs/go/presentation/App'",
    },
  ],
  ["nonliteral dynamic import", { "view.ts": "import(target)" }],
  [
    "CSS query loader",
    { "view.ts": 'import "./style.css?raw"', "style.css": "body {}" },
  ],
]) {
  test(`rejects ${name}`, () => {
    const result = runGuard(files);
    assert.equal(result.status, 1, result.output);
  });
}

test("rejects a source symlink even if it points at another TypeScript file", () => {
  const result = runGuard({ "view.ts": 'import "./escape"' }, (root) => {
    fs.writeFileSync(
      path.join(root, "outside.ts"),
      "window.go.presentation.App.Status()",
    );
    fs.symlinkSync(
      path.join(root, "outside.ts"),
      path.join(root, "frontend/src/escape.ts"),
    );
  });
  assert.equal(result.status, 1, result.output);
});

test("accepts real TS/TSX helpers, side-effect CSS, type declarations and canonical generated imports", () => {
  const result = runGuard({
    "main.tsx":
      'import React from "react"; import ReactDOM from "react-dom/client"; import App from "./App"; import "./style.css"; ReactDOM.createRoot(document.getElementById("root")!).render(<App />);',
    "App.tsx":
      'import {status} from "./presentation"; import {label} from "./helpers"; export default function App(){return <p>{label}</p>}',
    "helpers/index.ts": 'export const label = "synthetic";',
    "presentation.ts":
      'import {Status} from "../wailsjs/go/presentation/App"; import type {presentation} from "../wailsjs/go/models"; export const status = Status;',
    "style.css": "body { color: white; }",
    "vite-env.d.ts": '/// <reference types="vite/client" />',
  });
  assert.equal(result.status, 0, result.output);
});

for (const name of ["Observe", "ReadFile", "ExecuteSQL", "ClipboardGetText", "ClipboardSetText", "CopyText", "SaveFile", "SaveText", "SetBrandPath"]) {
  test(`rejects unapproved binding ${name}`, () => {
    const r = runGuard({
      "presentation.ts": `import {${name}} from '../wailsjs/go/presentation/App'`,
    });
    assert.equal(r.status, 1, r.output);
    assert.match(r.output, /unapproved Wails binding/);
  });
}
test("accepts explicit BF01 bindings", () => {
  const r = runGuard({
    "presentation.ts":
      "import {Status,Register,Login,Recover,Logout,CreateCompany,SelectCompany,SetTheme,RenewRecovery,CopyRecovery,AckRecovery,SaveRecovery,GetBrand,SaveBrand} from '../wailsjs/go/presentation/App'",
  });
  assert.equal(r.status, 0, r.output);
});

for (const code of [
  "const bindings = await import('../wailsjs/go/presentation/App'); bindings.ExecuteSQL('query')",
  "const {CopyText: copy} = await import('../wailsjs/go/presentation/App'); copy('text')",
  "import('../wailsjs/go/presentation/App').then(binding => binding.Status())",
  "import('../wailsjs/go/models')",
]) {
  test(`rejects dynamic canonical bindings causally: ${code}`, () => {
    // The same canonical adapter/static import is accepted; the dynamic
    // loader must fail by its nominal diagnostic, not missing paths/globals.
    const control = runGuard({
      "presentation.ts":
        "import {Status} from '../wailsjs/go/presentation/App'; export const status = Status;",
    });
    assert.equal(control.status, 0, control.output);
    const negative = runGuard({ "presentation.ts": code });
    assert.equal(negative.status, 1, negative.output);
    assert.match(
      negative.output,
      /dynamic Wails binding import bypasses nominal allowlist/,
    );
    assert.doesNotMatch(
      negative.output,
      /unparseable source|binding outside canonical|local module missing|direct or aliased runtime root/,
    );
  });
}
