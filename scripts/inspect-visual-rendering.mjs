import { chromium } from "../frontend/node_modules/@playwright/test/index.mjs";
import { preview } from "../frontend/node_modules/vite/dist/node/index.js";
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";
const root = process.cwd(),
  out =
    process.argv[2] ||
    fs.readFileSync("build/ui-021-visual/latest.txt", "utf8").trim(),
  manifest = JSON.parse(
    fs.readFileSync(path.join(out, "capture-manifest.json"), "utf8"),
  );
const referenceRoot = manifest.reference_root;
if (!referenceRoot || !fs.existsSync(path.join(referenceRoot, 'INFRADESKTOP-MOCKUPS-DESKTOP-V2.html')))
  throw Error('Fidelity NOT RUN: reference origin absent from capture manifest');
const server = await preview({
    root: path.join(root, "frontend"),
    configFile: false,
    build: { outDir: path.join(out, "fixture-bundle") },
    preview: { host: "127.0.0.1", port: 0 },
  }),
  addr = server.httpServer.address();
const browser = await chromium.launch({
  executablePath: "/usr/bin/google-chrome",
  headless: true,
});
const results = [];
try {
  for (const row of manifest.rows.filter((x) => x.viewport[0] === 1440 && x.viewport[1] === 860 && !x.variant)) {
    const ref = await browser.newPage({
      viewport: { width: 1440, height: 860 },
      deviceScaleFactor: 1,
    });
    await ref.goto(
      pathToFileURL(
        path.resolve(
          referenceRoot, "INFRADESKTOP-MOCKUPS-DESKTOP-V2.html",
        ),
      ).href +
        `#tela=${row.scene}&tema=${row.theme === "dark" ? "escuro" : "claro"}&cenario=${row.empty ? "vazio" : "piloto"}`,
    );
    await ref.evaluate(() => document.fonts.ready);
    await ref.evaluate(
      ({ alt }) => {
        if (alt)
          eval(
            "brand={name:'Aurora Telecom',accent:'violet',logo:''};draft={...brand};render()",
          );
        const app = document.querySelector("#app");
        document.body.append(app);
        app.style.position = "absolute";
        app.style.left = "0px";
        app.style.top = "-40px";
        app.style.width = "1440px";
        app.style.height = "900px";
        app.dataset.compact = "false";
        document.querySelector(".reviewbar").remove();
        document.querySelector("#viewport").remove();
      },
      { alt: row.alt },
    );
    await ref
      .locator(".client")
      .screenshot({
        path: path.join(out, row.key + "-reference-origin-aligned.png"),
      });
    const prod = await browser.newPage({
      viewport: { width: 1440, height: 860 },
      deviceScaleFactor: 1,
    });
    const q = new URLSearchParams({
      scene: String(row.scene),
      theme: row.theme,
    });
    if (row.empty) q.set("empty", "1");
    if (row.alt) q.set("alt", "1");
    await prod.goto(`http://127.0.0.1:${addr.port}/visual/index.html?${q}`);
    await prod.evaluate(() => document.fonts.ready);
    const read = async (page) =>
      page.evaluate(() => {
        const c = document.querySelector(".client").getBoundingClientRect();
        const selectors = [
          ".product-mark",
          ".product-mark svg",
          ".company-picker",
          ".topbar",
          ".page-body",
          ".traffic-chart",
          ".traffic-chart svg",
          ".chart-line",
          "linearGradient",
          "stop",
          ".settings-preview",
          ".auth-content",
          ".table-caption",
          ".app-table th",
          ".app-table td",
          ".subtitle",
          ".signal-source",
          ".list-footer",
          ".brand-logo-row",
          ".field input",
          ".swatch",
          ".theme-choice",
          ".preview-company",
          ".setting-section",
          ".settings-actions",
          ".company-intro p",
        ];
        return selectors.flatMap((sel) =>
          [...document.querySelectorAll(sel)].map((el) => {
            const r = el.getBoundingClientRect(),
              s = getComputedStyle(el);
            return {
              sel,
              x: r.x - c.x,
              y: r.y - c.y,
              width: r.width,
              height: r.height,
              font: s.font,
              lineHeight: s.lineHeight,
              color: s.color,
              background: s.backgroundColor,
              stroke: s.stroke,
              fill: s.fill,
              opacity: s.opacity,
              border: s.border,
              borderRadius: s.borderRadius,
              padding: s.padding,
              margin: s.margin,
              stopColor: s.stopColor,
              stopOpacity: s.stopOpacity,
              html:
                el.tagName === "path" || el.tagName === "stop"
                  ? el.outerHTML
                  : undefined,
            };
          }),
        );
      });
    results.push({
      key: row.key,
      reference: await read(ref),
      capture: await read(prod),
    });
    await ref.close();
    await prod.close();
  }
  fs.writeFileSync(
    path.join(out, "computed-style-investigation.json"),
    JSON.stringify(results, null, 2) + "\n",
  );
} finally {
  await browser.close();
  await new Promise((r) => server.httpServer.close(r));
}
