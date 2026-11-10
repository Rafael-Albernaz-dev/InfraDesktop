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
    fs.mkdirSync(path.join(root, "quality/baselines"), { recursive: true });
    fs.writeFileSync(
      path.join(root, "quality/baselines/frontend-layer-exceptions.json"),
      "[]\n",
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
        "import {Status} from '../../../wailsjs/go/presentation/App'",
    },
  ],
  [
    "binding below platform/wails",
    {
      "platform/wails/nested/radio.ts":
        "import {Status} from '../../../wailsjs/go/presentation/App'",
    },
  ],
  [
    "binding inside a feature",
    {
      "features/radio/radio-api.ts":
        "import {Status} from '../../../wailsjs/go/presentation/App'",
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
      'import {status} from "./platform/wails/presentation"; import {label} from "./helpers"; export default function App(){return <p>{label}</p>}',
    "helpers/index.ts": 'export const label = "synthetic";',
    "platform/wails/presentation.ts":
      'import {Status} from "../../../wailsjs/go/presentation/App"; import type {presentation} from "../../../wailsjs/go/models"; export const status = Status;',
    "style.css": "body { color: white; }",
    "vite-env.d.ts": '/// <reference types="vite/client" />',
  });
  assert.equal(result.status, 0, result.output);
});

for (const name of [
  "Observe",
  "ReadFile",
  "ExecuteSQL",
  "ClipboardGetText",
  "ClipboardSetText",
  "CopyText",
  "SaveFile",
  "SaveText",
  "SetBrandPath",
]) {
  test(`rejects unapproved binding ${name}`, () => {
    const r = runGuard({
      "platform/wails/presentation.ts": `import {${name}} from '../../../wailsjs/go/presentation/App'`,
    });
    assert.equal(r.status, 1, r.output);
    assert.match(r.output, /unapproved Wails binding/);
  });
}
test("accepts bindings in any capability file directly inside platform/wails", () => {
  const r = runGuard({
    "platform/wails/radio.ts":
      "import {GetRadio} from '../../../wailsjs/go/presentation/App'; import type {equipment} from '../../../wailsjs/go/models'; export const radio = GetRadio;",
    "platform/wails/presentation.ts": "export * from './radio';",
  });
  assert.equal(r.status, 0, r.output);
});
test("accepts explicit BF01 bindings", () => {
  const r = runGuard({
    "platform/wails/presentation.ts":
      "import {Status,Register,Login,Recover,Logout,CreateCompany,SelectCompany,SetTheme,RenewRecovery,CopyRecovery,AckRecovery,SaveRecovery,GetBrand,SaveBrand} from '../../../wailsjs/go/presentation/App'",
  });
  assert.equal(r.status, 0, r.output);
});

for (const code of [
  "const bindings = await import('../../../wailsjs/go/presentation/App'); bindings.ExecuteSQL('query')",
  "const {CopyText: copy} = await import('../../../wailsjs/go/presentation/App'); copy('text')",
  "import('../../../wailsjs/go/presentation/App').then(binding => binding.Status())",
  "import('../../../wailsjs/go/models')",
]) {
  test(`rejects dynamic canonical bindings causally: ${code}`, () => {
    // The same canonical adapter/static import is accepted; the dynamic
    // loader must fail by its nominal diagnostic, not missing paths/globals.
    const control = runGuard({
      "platform/wails/presentation.ts":
        "import {Status} from '../../../wailsjs/go/presentation/App'; export const status = Status;",
    });
    assert.equal(control.status, 0, control.output);
    const negative = runGuard({ "platform/wails/presentation.ts": code });
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

test("accepts nominal equipment bindings and rejects a secret getter", () => {
  const good = runGuard({
    "platform/wails/presentation.ts":
      "import {ListEquipment, GetEquipment, SaveEquipment, SetEquipmentActive, ListCredentials, PutCredential, InspectTrust, ConfirmTrust, ResetTrust, StartQuery, QueryStatus, CancelQuery, StartPPPoERead, PPPoEReadStatus, CancelPPPoERead, CancelPPPoEReadOperation, PPPoEReadPage, PPPoEContextCounts, ExportPPPoEOperational} from '../../../wailsjs/go/presentation/App';",
  });
  assert.equal(good.status, 0, good.output);
  const bad = runGuard({
    "platform/wails/presentation.ts":
      "import {GetSecret} from '../../../wailsjs/go/presentation/App';",
  });
  assert.notEqual(bad.status, 0);
});

test("accepts data properties named parent while rejecting global-root access and aliases", () => {
  const control = runGuard({
    "data.ts":
      'type Row = {parent?: string}; const row: Row = {parent: "VLAN"}; export const label = row.parent;',
  });
  assert.equal(control.status, 0, control.output);
  for (const code of [
    'const alias = parent; alias.postMessage("x", "*");',
    'const alias = window.parent; alias.postMessage("x", "*");',
    'const alias = globalThis; alias.parent.postMessage("x", "*");',
    'const {parent: alias} = window; alias.postMessage("x", "*");',
    "const row = {parent};",
  ]) {
    const result = runGuard({ "data.ts": code });
    assert.equal(result.status, 1, result.output);
    assert.match(result.output, /direct or aliased runtime root/);
  }
});

for (const target of ["features/radio", "app", "platform/wails"]) {
  test(`shared rejects dependency on ${target} while local shared imports remain valid`, () => {
    const files = {
      "shared/helper.ts": "export const value = 1;",
      [`${target}/helper.ts`]: "export const value = 1;",
    };
    const good = runGuard({
      ...files,
      "shared/view.ts": 'export {value} from "./helper";',
    });
    assert.equal(good.status, 0, good.output);
    for (const code of [
      `export {value} from "../${target}/helper";`,
      `import {value} from "../${target}/helper";`,
      `import("../${target}/helper");`,
      `export type Probe = typeof import("../${target}/helper");`,
    ]) {
      const bad = runGuard({ ...files, "shared/view.ts": code });
      assert.equal(bad.status, 1, bad.output);
      assert.match(bad.output, /shared dependency outside shared/);
      assert.doesNotMatch(
        bad.output,
        /local module missing|unparseable source/,
      );
    }
  });
}

test("rejects generated bindings outside the files directly inside platform/wails", () => {
  const spec = "../../../wailsjs/go/presentation/App";
  const code = `import {Status} from "${spec}";`;
  const good = runGuard({ "platform/wails/presentation.ts": code });
  assert.equal(good.status, 0, good.output);
  for (const name of [
    "features/access/view.ts",
    "shared/ui/view.ts",
    "platform/wails/nested/other.ts",
    "app/view.ts",
  ]) {
    const bad = runGuard({ [name]: code });
    assert.equal(bad.status, 1, bad.output);
    assert.match(bad.output, /binding outside canonical presentation adapter/);
  }
});

const layerEdge =
  "frontend/src/features/equipment/view.ts -> frontend/src/features/radio/helper.ts";
function withExceptions(entries) {
  return (root) =>
    fs.writeFileSync(
      path.join(root, "quality/baselines/frontend-layer-exceptions.json"),
      JSON.stringify(entries),
    );
}

test("rejects new upward and peer-layer edges, permits downward, same-feature and test imports", () => {
  for (const [from, to] of [
    ["equipment", "radio"],
    ["inventory", "monitoring"],
    ["monitoring", "inventory"],
  ]) {
    const files = { [`features/${to}/helper.ts`]: "export const value = 1;" };
    const spec = `../${to}/helper`;
    for (const code of [
      `import {value} from '${spec}';`,
      `export {value} from '${spec}';`,
      `import('${spec}');`,
      `export type Probe = typeof import('${spec}');`,
    ]) {
      const bad = runGuard({ ...files, [`features/${from}/view.ts`]: code });
      assert.equal(bad.status, 1, bad.output);
      assert.match(bad.output, /feature layer dependency/);
      assert.doesNotMatch(
        bad.output,
        /local module missing|unparseable source/,
      );
      const good = runGuard({
        ...files,
        [`features/${from}/view.test.tsx`]: code,
      });
      assert.equal(good.status, 0, good.output);
    }
  }
  const good = runGuard({
    "features/access/helper.ts": "export const value = 1;",
    "features/radio/helper.ts": 'export {value} from "../access/helper";',
    "features/radio/view.ts": 'export {value} from "./helper";',
  });
  assert.equal(good.status, 0, good.output);
});

test("accepts a frozen edge and rejects it when resolved or only present in tests", () => {
  const files = {
    "features/radio/helper.ts": "export const value = 1;",
    "features/equipment/view.ts": 'export {value} from "../radio/helper";',
  };
  const good = runGuard(files, withExceptions([layerEdge]));
  assert.equal(good.status, 0, good.output);
  for (const replacement of [
    { "features/equipment/view.ts": "export const value = 1;" },
    {
      "features/equipment/view.test.tsx": files["features/equipment/view.ts"],
      "features/equipment/view.ts": "",
    },
  ]) {
    const bad = runGuard(
      { ...files, ...replacement },
      withExceptions([layerEdge]),
    );
    assert.equal(bad.status, 1, bad.output);
    assert.match(bad.output, /stale frontend layer exception/);
  }
});

test("only app and main import app in production, including type and dynamic imports", () => {
  for (const code of [
    'import {value} from "../../app/helper";',
    'export {value} from "../../app/helper";',
    'import("../../app/helper");',
    'export type Probe = typeof import("../../app/helper");',
  ]) {
    for (const area of ["features/equipment", "platform/wails"]) {
      const files = { "app/helper.ts": "export const value = 1;" };
      const bad = runGuard({ ...files, [`${area}/view.ts`]: code });
      assert.equal(bad.status, 1, bad.output);
      assert.match(bad.output, /app dependency outside app or main/);
      assert.doesNotMatch(
        bad.output,
        /local module missing|unparseable source/,
      );
      const good = runGuard({ ...files, [`${area}/view.test.tsx`]: code });
      assert.equal(good.status, 0, good.output);
    }
  }
  const good = runGuard({
    "app/helper.ts": "export const value = 1;",
    "app/view.ts": 'export {value} from "./helper";',
    "main.tsx": 'export {value} from "./app/helper";',
  });
  assert.equal(good.status, 0, good.output);
});

test("CSS side-effect imports obey app and feature-layer boundaries", () => {
  for (const [target, diagnostic] of [
    ["app", /app dependency outside app or main/],
    ["features/radio", /feature layer dependency/],
    ["features/monitoring", /feature layer dependency/],
  ]) {
    const source = target === "features/monitoring" ? "inventory" : "equipment";
    const code = `import '../../${target}/style.css';`;
    const result = runGuard({
      [`${target}/style.css`]: "body {}",
      [`features/${source}/view.ts`]: code,
    });
    assert.equal(result.status, 1, result.output);
    assert.match(result.output, diagnostic);
    assert.doesNotMatch(
      result.output,
      /CSS must|local module missing|unparseable source/,
    );
  }
  const good = runGuard({
    "features/access/style.css": "body {}",
    "features/equipment/view.ts": "import '../access/style.css';",
  });
  assert.equal(good.status, 0, good.output);
});
