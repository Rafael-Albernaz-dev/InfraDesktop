import assert from "node:assert/strict";
import { execFileSync, spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const guard = fileURLToPath(new URL("./backend-layout.mjs", import.meta.url));

function runGuard(t, paths, exceptions = []) {
  const cwd = mkdtempSync(join(tmpdir(), "backend-layout-"));
  t.after(() => rmSync(cwd, { recursive: true, force: true }));
  // Git hooks can export repository-local paths, including the index.
  const env = Object.fromEntries(
    Object.entries(process.env).filter(([name]) => !name.startsWith("GIT_")),
  );
  execFileSync("git", ["init", "-q"], { cwd, env });
  for (const path of paths) {
    const file = join(cwd, path);
    mkdirSync(dirname(file), { recursive: true });
    writeFileSync(file, "fixture");
  }
  const baseline = join(
    cwd,
    "quality/baselines/backend-layout-exceptions.json",
  );
  mkdirSync(dirname(baseline), { recursive: true });
  writeFileSync(baseline, JSON.stringify(exceptions));
  execFileSync("git", ["add", "."], { cwd, env });
  return spawnSync(process.execPath, [guard], { cwd, env, encoding: "utf8" });
}

test("rejects a newly tracked file outside data directories", (t) => {
  const result = runGuard(t, ["internal/adapter/response.txt"]);
  assert.equal(result.status, 1);
  assert.match(result.stderr, /internal\/adapter\/response\.txt: move/);
});

test("accepts fixtures and migrations in their own directories", (t) => {
  const result = runGuard(t, [
    "internal/adapter/testdata/nested/response.txt",
    "internal/adapter/migrations/schema.sql",
  ]);
  assert.equal(result.status, 0, result.stderr);
});

test("rejects a stale exception while honoring an existing one", (t) => {
  const result = runGuard(
    t,
    ["internal/adapter/schema.sql"],
    ["internal/adapter/deleted.sql", "internal/adapter/schema.sql"],
  );
  assert.equal(result.status, 1);
  assert.match(
    result.stderr,
    /internal\/adapter\/deleted\.sql: remove obsolete/,
  );
  assert.doesNotMatch(result.stderr, /schema\.sql/);
});
