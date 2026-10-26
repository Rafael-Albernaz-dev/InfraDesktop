// Test-only entry imports the same React components; never replaces product bindings.
import { chromium } from "../frontend/node_modules/@playwright/test/index.mjs";
import {
  build,
  preview,
} from "../frontend/node_modules/vite/dist/node/index.js";
import react from "../frontend/node_modules/@vitejs/plugin-react/dist/index.js";
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { pathToFileURL } from "node:url";
import { inspectActions, expectedActions } from "./visual-actions.mjs";
import { runChecked, assertRendering } from "./visual-checks.mjs";
const root = process.cwd(),
  out = path.resolve(
    "build/ui-021-visual",
    new Date().toISOString().replaceAll(":", "-"),
  );
fs.mkdirSync(out, { recursive: true });
const ref = path.resolve(
  process.env.INFRADESKTOP_VISUAL_REFERENCES ||
    "docs/reference/infradesktop-ui-0.2.1",
);
const hasRef = fs.existsSync(path.join(ref, "mockups-v2/manifesto.json"));
const sha = (p) =>
  crypto.createHash("sha256").update(fs.readFileSync(p)).digest("hex");
function files(p) {
  return fs
    .readdirSync(p, { withFileTypes: true })
    .flatMap((x) =>
      x.isDirectory() ? files(path.join(p, x.name)) : [path.join(p, x.name)],
    );
}
const inputs = [
  ...files("frontend/src"),
  ...files("frontend/visual"),
  ...files("frontend/public/fonts"),
  "frontend/package-lock.json",
  "frontend/vite.config.ts",
  "scripts/test-visual.mjs",
  "scripts/compare-visual.py",
  "scripts/inspect-visual-rendering.mjs",
  "scripts/visual-checks.mjs",
  "scripts/visual-actions.mjs",
  ...files("frontend/dist"),
].map((p) => ({ path: path.resolve(p), sha256: sha(p) }));
if (hasRef) {
  const html = path.join(ref, "INFRADESKTOP-MOCKUPS-DESKTOP-V2.html"),
    manifest = path.join(ref, "mockups-v2/manifesto.json");
  if (
    sha(html) !==
      "975e8c4a9c4e00e70735dd21e62736af78ea40c52e731100df0d4e8599e120d5" ||
    sha(manifest) !==
      "07ad224cd454af7cf3367819d360740696d57dce3590662c7e5627cdb8514e68"
  )
    throw Error("Approved reference identity changed");
  for (const x of JSON.parse(fs.readFileSync(manifest, "utf8")).images)
    if (sha(path.join(ref, x.file)) !== x.sha256)
      throw Error("Reference PNG changed: " + x.file);
  inputs.push(...[html, manifest].map((p) => ({ path: p, sha256: sha(p) })));
}
const bundle = path.join(out, "fixture-bundle");
await build({
  root: path.join(root, "frontend"),
  configFile: false,
  plugins: [react()],
  build: {
    outDir: bundle,
    emptyOutDir: true,
    rollupOptions: { input: path.join(root, "frontend/visual/index.html") },
  },
});
inputs.push(...files(bundle).map((p) => ({ path: p, sha256: sha(p) })));
const server = await preview({
  root: path.join(root, "frontend"),
  configFile: false,
  build: { outDir: bundle },
  preview: { host: "127.0.0.1", port: 0, strictPort: true },
});
const address = server.httpServer.address();
const browser = await chromium.launch({
  executablePath: "/usr/bin/google-chrome",
  headless: true,
});
const states = [];
for (const theme of ["light", "dark"])
  for (let scene = 1; scene <= 7; scene++) {
    const stems = [
      "",
      "01-acesso-local",
      "02-empresas",
      "03-dashboard",
      "04-equipamentos",
      "05-concentrador",
      "06-topologia",
      "07-aparencia",
    ];
    const pt = theme === "dark" ? "escuro" : "claro";
    states.push({ scene, theme, stem: stems[scene] + "-" + pt });
    if (scene === 3 || scene === 4)
      states.push({
        scene,
        theme,
        empty: true,
        stem: stems[scene] + "-" + pt + "-primeiro-uso",
      });
  }
for (const scene of [7, 3])
  states.push({
    scene,
    theme: "dark",
    alt: true,
    stem:
      (scene === 7 ? "07-aparencia" : "03-dashboard") +
      "-marca-alternativa-escuro",
  });
for(const theme of ['light','dark']) {
 for(const variant of ['register','recover','invalid','unavailable','code'])states.push({scene:1,theme,variant,stem:'bf04-access-'+variant+'-'+theme});
 for(const variant of ['security','security-rejected','security-unknown','security-code','menu','navigation'])states.push({scene:3,theme,variant,empty:true,stem:'bf04-'+variant+'-'+theme});
 states.push({scene:7,theme,variant:'no-company',stem:'bf04-appearance-no-company-'+theme});
}
const rows = [];const contrasts=[];
let page;
try {
  const matrix=[[1440,900],[900,1440],[768,1024],[1366,768],[1280,720],[1100,700],[1440,860],[720,600]];
  const requested=process.env.INFRADESKTOP_VISUAL_VIEWPORTS?.split(',');
  if(requested?.some(key=>!matrix.some(([w,h])=>`${w}x${h}`===key)))throw Error('Unknown requested viewport');
  const selected=requested?matrix.filter(([w,h])=>requested.includes(`${w}x${h}`)):matrix;
  for (const [width, height] of selected)
    for (const state of states) {
      const key = state.stem + "-" + width + "x" + height;
      page = await browser.newPage({
        viewport: { width, height },
        deviceScaleFactor: 1,
      });
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      const params = new URLSearchParams({
        scene: String(state.scene),
        theme: state.theme,
      });
      if(state.variant)params.set('variant',state.variant);
      if (state.empty) params.set("empty", "1");
      if (state.alt) params.set("alt", "1");
      await page.goto(
        `http://127.0.0.1:${address.port}/visual/index.html?${params}`,
      );
      await page.locator(".app").waitFor();
      await page.evaluate(() => document.fonts.ready);
      if(state.variant==='menu')await page.getByRole('button',{name:'Conta: Operador'}).click();
      if(state.variant==='navigation'){if(width<1000)await page.getByRole('button',{name:'Navegação',exact:true}).click();await page.getByRole('button',{name:'Configurações',exact:true}).click();}
      const bounds = await page.evaluate(() => ({
        width: document.documentElement.scrollWidth,
        height: document.documentElement.scrollHeight,
        sidebar: document.querySelector(".sidebar")?.getBoundingClientRect()
          .width,
        header: document.querySelector(".topbar")?.getBoundingClientRect()
          .height,
        inspector: document.querySelector(".inspector")?.getBoundingClientRect()
          .width,
        font: getComputedStyle(document.body).fontFamily,
        fontSize: getComputedStyle(document.body).fontSize,
        textColor: getComputedStyle(document.querySelector(".app")).color,
        zoom: devicePixelRatio,
        fonts: [...document.fonts].map((f) => ({
          family: f.family,
          weight: f.weight,
          status: f.status,
        })),
      }));
      assertRendering(
        bounds,
        { width, height, navigationOpen:state.variant==='navigation', scene: state.scene, theme: state.theme, inspectorExpected: [4,5,6].includes(state.scene) && !state.empty },
        errors,
      );
      if (state.scene === 1) {
        await page
          .getByRole("button", {
            name: state.theme === "dark" ? "Escuro" : "Claro",
            exact: true,
          })
          .focus();
        if (
          (await page
            .getByRole("button", {
              name: state.theme === "dark" ? "Escuro" : "Claro",
              exact: true,
            })
            .getAttribute("aria-pressed")) !== "true"
        )
          throw Error("access theme selection");
      }
      const reachability=await page.evaluate(inspectActions,expectedActions(state,width));
      if(reachability.failures.length)throw Error('Unreachable actions '+key+': '+JSON.stringify(reachability));
      if(state.scene===7&&!state.variant&&width===1440&&height===860) {
        const palette=await page.evaluate(()=>{
          const app=document.querySelector('.app');const original=app.dataset.accent;
          const rgb=c=>{const probe=document.createElement('span');probe.style.color=c;app.append(probe);const parts=getComputedStyle(probe).color.match(/[\d.]+/g).slice(0,3).map(Number);probe.remove();return parts;};
          const luminance=c=>rgb(c).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((v,n,i)=>v+n*[.2126,.7152,.0722][i],0);
          const ratio=(a,b)=>{const x=luminance(a),y=luminance(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05)};
          const rows=[];for(const id of ['teal','violet','copper','blue','green','rose','indigo','slate']){app.dataset.accent=id;const css=getComputedStyle(app);const p=css.getPropertyValue('--primary'),on=css.getPropertyValue('--on-primary'),tint=css.getPropertyValue('--tint'),page=css.getPropertyValue('--page');rows.push({id,button:ratio(p,on),tint:ratio(p,tint),focus:ratio(p,page),error:ratio(css.getPropertyValue('--danger'),page)});}
          app.dataset.accent=original;return rows;
        });
        for(const row of palette)if(row.button<4.5||row.tint<4.5||row.focus<3||row.error<4.5)throw Error('palette contrast '+JSON.stringify(row));
        contrasts.push({theme:state.theme,palette});
      }
      const capture = path.join(out, key + ".png");
      await page.screenshot({ path: capture });
      let reference = null,
        crop = null;
      if (hasRef) {
        if (width === 1440 && height===860 && !state.variant) {
          reference = path.join(ref, "mockups-v2/telas", state.stem + ".png");
          crop = [0, 40, 1440, 860];
        }
        {
          const refPage = await browser.newPage({
            viewport: { width, height },
            deviceScaleFactor: 1,
          });
          await refPage.goto(
            pathToFileURL(
              path.join(ref, "INFRADESKTOP-MOCKUPS-DESKTOP-V2.html"),
            ).href +
              `#tela=${state.scene}&tema=${state.theme === "dark" ? "escuro" : "claro"}&cenario=${state.empty ? "vazio" : "piloto"}`,
          );
          await refPage.evaluate(() => document.fonts.ready);
          await refPage.evaluate(
            ({ width, height, alt }) => {
              const app = document.querySelector("#app");
              document.body.append(app);
              app.style.position='absolute';app.style.left='0px';app.style.top='-40px';
              app.style.width = width + "px";
              app.style.height = height + 40 + "px";
              if (alt)
                eval(
                  "brand={name:'Aurora Telecom',accent:'violet',logo:''};draft={...brand};render()",
                );
              app.style.width = width + "px";
              app.style.height = height + 40 + "px";
              app.dataset.compact=String(width<1250);
            },
            { width, height, alt: state.alt },
          );
          const rendered = path.join(out, key + "-reference-rendered.png");
          await refPage.locator(".client").screenshot({ path: rendered });
          if (width !== 1440 || height!==860 || state.variant) reference = rendered;
          await refPage.close();
        }
      }
      rows.push({
        ...state,
        key,
        viewport: [width, height],
        capture,
        reference,
        crop,
        bounds,
        reachability,
        reference_relation: state.variant ? "Historical context only: BF04 variant has no original same-state oracle; review against CONTRACT.md" : "Historical same scene/theme; authorized BF04 shell/reflow delta",
        authorized_delta: state.variant || "BF04 shell/reflow/personalization",
        errors,
        capture_sha256: sha(capture),
        reference_sha256: reference ? sha(reference) : null,
      });
      if(state.variant==='navigation'&&width<1000){await page.getByRole('button',{name:'Fechar navegação',exact:true}).click();if(await page.getByRole('button',{name:'Navegação',exact:true}).getAttribute('aria-expanded')!=='false')throw Error('navigation did not close');}
      // Interaction proofs use actual React state and the same handlers as production.
      if (width === 1440 && height===860 && state.theme === "light" && !state.empty && !state.variant) {
        if (state.scene === 4) {
          await page.getByLabel("Buscar equipamento").fill("AP Serra");
          if ((await page.locator("tbody tr").count()) !== 1)
            throw Error("filter");
          await page.locator("tbody tr").click();
          await page
            .locator(".inspector h2")
            .filter({ hasText: "AP Serra" })
            .waitFor();
          await page.getByLabel("Buscar equipamento").fill("");
          await page.getByLabel("Estado dos equipamentos").selectOption("bad");
          if ((await page.locator("tbody tr").count()) !== 1)
            throw Error("state filter");
        }
        if (state.scene === 5) {
          await page
            .getByRole("button", { name: "Origem da leitura", exact: true })
            .click();
          await page
            .getByRole("dialog")
            .getByText("BNG Centro · SSH · 14:32:18", { exact: true })
            .waitFor();
          await page
            .getByRole("dialog")
            .getByText("AP Serra · 14:32:05", { exact: true })
            .waitFor();
          await page.keyboard.press("Escape");
          if (
            !(await page
              .getByRole("tab", { name: "Interfaces", exact: true })
              .isDisabled())
          )
            throw Error("future tab enabled");
          await page
            .getByRole("row")
            .filter({ hasText: "assinante_003" })
            .click();
          await page.getByText("Não associado", { exact: true }).waitFor();
          if (await page.locator(".signal-value").count())
            throw Error("absent radio synthesized");
          await page.getByRole("tab", { name: "Resumo", exact: true }).click();
          await page.getByText("18 dias, 7 horas").waitFor();
          await page.getByRole("tab", { name: "PPPoE", exact: true }).click();
          await page.getByLabel("Buscar sessão").fill("assinante_010");
          if ((await page.locator("tbody tr:visible").count()) !== 1)
            throw Error("session filter");
        }
        if (state.scene === 6) {
          await page.getByRole("button", { name: /AP Vale/ }).click();
          await page
            .locator(".inspector h2")
            .filter({ hasText: "AP Vale" })
            .waitFor();
        }
        if (state.scene === 7) {
          await page.locator("input[type=file]").setInputFiles({
            name: "invalid.svg",
            mimeType: "image/svg+xml",
            buffer: Buffer.from("<svg/>"),
          });
          await page
            .getByRole("alert")
            .filter({ hasText: "Escolha uma imagem" })
            .waitFor();
          await page.getByLabel("Nome visual", { exact: true }).fill("Draft");
          await page
            .getByRole("button", { name: "Violeta", exact: true })
            .click();
          await page
            .getByRole("button", { name: "Cancelar", exact: true })
            .click();
          if (
            (await page.getByLabel("Nome visual", { exact: true }).inputValue()) !==
            "SerraLink Telecom"
          )
            throw Error("cancel");
          await page
            .getByLabel("Nome visual", { exact: true })
            .fill("Restorable draft");
          await page
            .getByRole("button", { name: "Restaurar padrão", exact: true })
            .click();
          if (
            (await page.getByLabel("Nome visual", { exact: true }).inputValue()) !==
            ""
          )
            throw Error("restore name");
          await page
            .getByRole("button", { name: "Cancelar", exact: true })
            .click();
          await page.getByLabel("Nome visual", { exact: true }).fill("Aurora Telecom");
          await page
            .getByRole("button", { name: "Salvar", exact: true })
            .click();
          await page
            .getByRole("button", { name: "Selecionar empresa" })
            .filter({ hasText: "Aurora Telecom" })
            .waitFor();
        }
        if (state.scene === 2) {
          await page
            .getByRole("button", { name: "Nova empresa", exact: true })
            .click();
          await page.getByRole("dialog").waitFor();
          await page.keyboard.press("Escape");
          if (await page.getByRole("dialog").count()) throw Error("escape");
        }
      }
      await page.close();
      page = null;
    }
  for (const input of inputs)
    if (sha(input.path) !== input.sha256)
      throw Error("STALE input changed during capture: " + input.path);
  const manifest = {
    reference_root: hasRef ? ref : null,
    version: "0.4.0",
    matrix: {selected, full:matrix, complete:selected.length===matrix.length},
    fixture: true,
    runtime: "Chromium " + browser.version(),
    DPR: 1,
    zoom: 1,
    at: new Date().toISOString(),
    inputs,
    contrasts,
    rows,
    capture: "PASS",
    fidelity: hasRef ? "REQUIRES_REVIEW" : "NOT RUN: private references absent",
  };
  fs.writeFileSync(
    path.join(out, "capture-manifest.json"),
    JSON.stringify(manifest, null, 2) + "\n",
  );
  fs.writeFileSync("build/ui-021-visual/latest.txt", out + "\n");
  if (hasRef) {
    runChecked("render investigation", "node", [
      "scripts/inspect-visual-rendering.mjs",
      out,
    ]);
    runChecked("comparison", "python3", [
      "scripts/compare-visual.py",
      "--captures",
      out,
      "--output",
      path.join(out, "comparison"),
    ]);
  }
  console.log(
    `Capture/smoke PASS: ${rows.length} states. Fidelity: ${hasRef ? "REQUIRES VISUAL REVIEW" : "NOT RUN (private references absent)"}. ${out}`,
  );
} finally {
  if (page) await page.close();
  await browser.close();
  await new Promise((r) => server.httpServer.close(r));
}
