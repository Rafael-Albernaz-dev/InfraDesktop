import ts from "../frontend/node_modules/typescript/lib/typescript.js";
import fs from "node:fs";
import path from "node:path";
const sourceRoot = path.resolve("frontend/src");
const presentation = path.join(sourceRoot, "presentation.ts");
const entrypoint = path.join(sourceRoot, "main.tsx");
const bridgeRoots = new Set([
  "window",
  "globalThis",
  "self",
  "parent",
  "top",
  "frames",
  "go",
  "runtime",
]);
const genericAPIs = new Set([
  "eval",
  "Function",
  "fetch",
  "WebSocket",
  "XMLHttpRequest",
  "require",
]);
const externalModules = new Set([
  "react",
  "react-dom/client",
  "react-dom",
  "vitest",
  "@testing-library/react",
  "@testing-library/jest-dom/vitest",
]);

function insideSource(file) {
  return file.startsWith(sourceRoot + path.sep);
}
function regularFile(file) {
  try {
    return fs.lstatSync(file).isFile() && insideSource(fs.realpathSync(file));
  } catch {
    return false;
  }
}
function inspect(name, code) {
  const filename = path.resolve(name);
  const source = ts.createSourceFile(
    name,
    code,
    ts.ScriptTarget.Latest,
    true,
    name.endsWith("tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const problems = source.parseDiagnostics.map(() => "unparseable source");
  function check(spec, sideEffect = false) {
    if (spec.includes("wailsjs")) {
      if (
        filename !== presentation ||
        !["../wailsjs/go/presentation/App", "../wailsjs/go/models"].includes(
          spec,
        )
      ) {
        problems.push("binding outside canonical presentation adapter");
      }
      return;
    }
    if (!spec.startsWith(".")) {
      if (!externalModules.has(spec))
        problems.push("external capability: " + spec);
      return;
    }
    const resolved = path.resolve(path.dirname(filename), spec);
    if (!insideSource(resolved) || /[?#\0]/.test(spec)) {
      problems.push(
        "local import outside inspected source tree or unsupported loader",
      );
      return;
    }
    const ext = path.extname(resolved);
    if (ext === ".css") {
      if (!sideEffect || !regularFile(resolved))
        problems.push("CSS must be a regular local side-effect asset");
      return;
    }
    if (ext && ext !== ".ts" && ext !== ".tsx") {
      problems.push("uninspected local module format: " + spec);
      return;
    }
    const candidates = ext
      ? [resolved]
      : [
          resolved + ".ts",
          resolved + ".tsx",
          path.join(resolved, "index.ts"),
          path.join(resolved, "index.tsx"),
        ];
    if (!candidates.some(regularFile))
      problems.push("local module missing or unsafe: " + spec);
  }
  function visit(n) {
    if (ts.isImportDeclaration(n) || ts.isExportDeclaration(n)) {
      if (
        n.moduleSpecifier &&
        n.moduleSpecifier.text === "../wailsjs/go/presentation/App"
      ) {
        const approved = new Set([
          "StartPPPoECollection",
          "StartPPPoECatalog",
          "StartPPPoERadius",
          "StartPPPoEPing",
          "PPPoEDiagnosticStatus",
          "CancelPPPoEDiagnostic",
          "GetPPPoEDiagnostic",
          "ListPPPoEDiagnostic",
          "PPPoECollectionStatus",
          "CancelPPPoECollection",
          "GetPPPoECurrent",
          "GetPPPoEObservation",
          "ListPPPoESessions",
          "GetPPPoESession",
          "SavePPPoEReference",
          "ListPPPoEReferences",
          "DeletePPPoEReference",
          "ComparePPPoE",
          "ExportPPPoE",
          "Status",
          "Register",
          "Login",
          "Recover",
          "Logout",
          "CreateCompany",
          "SelectCompany",
          "SetTheme",
          "RenewRecovery",
          "CopyRecovery",
          "AckRecovery",
          "SaveRecovery",
          "GetBrand",
          "SaveBrand",
          "CancelQuery",
          "ConfirmTrust",
          "ResetTrust",
          "GetEquipment",
          "EnrollmentStatus",
          "InspectTrust",
          "ListCredentials",
          "ListEquipment",
          "PutCredential",
          "QueryStatus",
          "SaveEquipment",
          "SetEquipmentActive",
          "StartQuery",

        ]);
        const clause = n.importClause;
        if (
          !ts.isImportDeclaration(n) ||
          !clause ||
          clause.name ||
          !clause.namedBindings ||
          !ts.isNamedImports(clause.namedBindings) ||
          clause.namedBindings.elements.some(
            (e) => !approved.has((e.propertyName || e.name).text),
          )
        )
          problems.push("unapproved Wails binding");
      }
      if (n.moduleSpecifier)
        check(
          n.moduleSpecifier.text,
          ts.isImportDeclaration(n) && !n.importClause,
        );
    }
    if (ts.isImportEqualsDeclaration(n))
      problems.push("unsupported import assignment");
    if (
      ts.isCallExpression(n) &&
      n.expression.kind === ts.SyntaxKind.ImportKeyword
    ) {
      if (n.arguments.length !== 1 || !ts.isStringLiteral(n.arguments[0]))
        problems.push("dynamic unbounded import");
      else if (n.arguments[0].text.includes("wailsjs"))
        problems.push("dynamic Wails binding import bypasses nominal allowlist");
      else check(n.arguments[0].text);
    }
    if (ts.isMetaProperty(n) || n.kind === ts.SyntaxKind.ThisKeyword)
      problems.push("uninspected global or module loader access");
    if (ts.isIdentifier(n)) {
      // A member/type/object key names data, not a global variable reference.
      // Keep expression roots, shorthand and destructuring bindings fail-closed;
      // window.parent still visits/rejects the window expression independently.
      const memberName =
        (ts.isPropertyAccessExpression(n.parent) && n.parent.name === n) ||
        (ts.isPropertySignature(n.parent) && n.parent.name === n) ||
        (ts.isPropertyAssignment(n.parent) && n.parent.name === n);
      // This is an architectural subset, not a sandbox for arbitrary hostile JS.
      if (bridgeRoots.has(n.text) && !memberName)
        problems.push("direct or aliased runtime root");
      if (genericAPIs.has(n.text))
        problems.push("generic execution/network API");
      if (n.text === "document") {
        const access = n.parent;
        const call = access?.parent;
        const mounting =
          filename === entrypoint &&
          ts.isPropertyAccessExpression(access) &&
          access.expression === n &&
          access.name.text === "getElementById" &&
          ts.isCallExpression(call) &&
          call.expression === access;
        if (!mounting) problems.push("DOM global outside narrow root mount");
      }
    }
    ts.forEachChild(n, visit);
  }
  visit(source);
  return problems;
}
for (const [name, code, bad] of [
  ["x.ts", "import fs from 'node:fs'", true],
  ["helper.ts", "import {exec} from 'node:child_process'", true],
  ["view.ts", "import {Status} from '../wailsjs/go/presentation/App'", true],
  ["view.ts", "window.go.presentation.App.Status()", true],
  ["view.ts", "import(target)", true],
  ["x.ts", "import React from 'react'", false],
]) {
  const got = inspect(name, code);
  if (Boolean(got.length) !== bad)
    throw Error("frontend guard fixture failed: " + code);
  console.log("frontend fixture:", name, "rejected=", Boolean(got.length));
}
const files = [];
let bad = false;
function collect(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isSymbolicLink()) {
      console.error(p, ["source symlink is outside the inspected subset"]);
      bad = true;
    } else if (e.isDirectory()) collect(p);
    else if (e.isFile() && /\.tsx?$/.test(p)) files.push(p);
    else if (!e.isFile() || path.extname(p) !== ".css") {
      console.error(p, [
        "unsupported source format; allow only inspected TS/TSX and CSS assets",
      ]);
      bad = true;
    }
  }
}
if (fs.lstatSync(sourceRoot).isSymbolicLink()) {
  console.error("source root must not be a symlink");
  process.exit(1);
}
collect(sourceRoot);
// Every authored TS/TSX helper is inspected, including unreachable files and tests.
for (const f of files) {
  const issues = inspect(f, fs.readFileSync(f, "utf8"));
  if (issues.length) {
    console.error(f, issues);
    bad = true;
  }
}
if (bad) process.exit(1);
console.log(
  "ARQ-001 frontend source/helper imports and binding boundary: PASS",
);
