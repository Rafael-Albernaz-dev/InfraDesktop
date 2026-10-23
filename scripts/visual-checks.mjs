import { spawnSync } from "node:child_process";

export function runChecked(label, executable, args, options = {}) {
  const result = spawnSync(executable, args, { stdio: "inherit", ...options });
  if (result.error || result.signal || result.status !== 0) {
    const reason =
      result.error?.code || result.signal || `exit ${result.status}`;
    throw new Error(`${label} failed: ${reason}`);
  }
  return result;
}

export function assertRendering(
  bounds,
  { width, height, scene, theme, inspectorExpected, navigationOpen },
  errors,
) {
  const expectedColor =
    theme === "dark" ? "rgb(238, 238, 241)" : "rgb(32, 33, 36)";
  const invalid =
    typeof inspectorExpected !== "boolean" ||
    bounds.width !== width ||
    bounds.height !== height ||
    bounds.zoom !== 1 ||
    !bounds.font.includes("Roboto") ||
    bounds.fontSize !== "15px" ||
    bounds.textColor !== expectedColor ||
    errors.length ||
    (scene !== 1 &&
      (bounds.sidebar !== (width<1000?(navigationOpen?224:0):width < 1250 ? 200 : 224) ||
        bounds.header !== 60)) ||
    (inspectorExpected
      ? bounds.inspector !== (width<1100 ? width-(width<1000?0:200) : width < 1250 ? 268 : 300)
      : bounds.inspector !== undefined) ||
    !bounds.fonts.some(
      (font) => font.family === "Roboto" && font.status === "loaded",
    );
  if (invalid)
    throw new Error(
      `Unexpected rendering: ${JSON.stringify({ bounds, scene, theme, errors })}`,
    );
}
