import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";

const exceptions = JSON.parse(
  readFileSync("quality/baselines/backend-layout-exceptions.json", "utf8"),
);
if (
  !Array.isArray(exceptions) ||
  exceptions.some(
    (path, index) =>
      typeof path !== "string" || (index > 0 && exceptions[index - 1] >= path),
  )
) {
  throw new Error(
    "backend layout exceptions must be a sorted array of unique paths",
  );
}

const paths = execFileSync("git", ["ls-files", "-z", "--", "internal/"], {
  encoding: "utf8",
})
  .split("\0")
  .filter(Boolean);
const misplaced = new Set(
  paths.filter(
    (path) => !path.endsWith(".go") && !/\/(testdata|migrations)\//.test(path),
  ),
);
const allowed = new Set(exceptions);

for (const path of misplaced) {
  if (allowed.has(path)) continue;
  console.error(`${path}: move non-Go file into testdata/ or migrations/`);
  process.exitCode = 1;
}
for (const path of exceptions) {
  if (misplaced.has(path) && existsSync(path)) continue;
  console.error(`${path}: remove obsolete backend layout exception`);
  process.exitCode = 1;
}
