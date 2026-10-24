// Focal composition evidence only; the real distributed journey is a separate gate.
import { chromium } from "../frontend/node_modules/@playwright/test/index.mjs";
import {
  build,
  preview,
} from "../frontend/node_modules/vite/dist/node/index.js";
import react from "../frontend/node_modules/@vitejs/plugin-react/dist/index.js";
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import assert from "node:assert/strict";
import { pathToFileURL } from "node:url";
import { inspectActions } from "./visual-actions.mjs";
const out = path.resolve("docs/evidence/l04-l05-cofre-ux/visual");
fs.mkdirSync(out, { recursive: true });
const bundle = path.resolve("build/cofre-ux-visual-bundle");
await build({
  root: path.resolve("frontend"),
  configFile: false,
  plugins: [react()],
  build: {
    outDir: bundle,
    emptyOutDir: true,
    rollupOptions: { input: path.resolve("frontend/visual/equipment.html") },
  },
});
const server = await preview({
  root: path.resolve("frontend"),
  configFile: false,
  build: { outDir: bundle },
  preview: { host: "127.0.0.1", port: 0 },
});
const port = server.httpServer.address().port;
const browser = await chromium.launch({
  executablePath: "/usr/bin/google-chrome",
  headless: true,
});
const rows = [];
const files = (dir) =>
  fs
    .readdirSync(dir, { withFileTypes: true })
    .flatMap((e) =>
      e.isDirectory()
        ? files(path.join(dir, e.name))
        : [path.join(dir, e.name)],
    );
const sha = (p) =>
  crypto.createHash("sha256").update(fs.readFileSync(p)).digest("hex");
const referenceHTML = path.resolve(
  "docs/reference/infradesktop-ui-0.2.1/INFRADESKTOP-MOCKUPS-DESKTOP-V2.html",
);
assert.equal(
  sha(referenceHTML),
  "975e8c4a9c4e00e70735dd21e62736af78ea40c52e731100df0d4e8599e120d5",
);
try {
  for (const [width, height] of [
    [1440, 860],
    [720, 600],
  ])
    for (const theme of ["light", "dark"])
      for (const state of [
        "empty",
        "list",
        "no-results",
        "inactive-only",
        "vault",
        "vault-form",
        "device-form",
        "existing-form",
        "credential-form",
        "trust",
      ]) {
        const key = `${state}-${theme}-${width}x${height}`;
        const page = await browser.newPage({
          viewport: { width, height },
          deviceScaleFactor: 1,
        });
        const errors = [];
        page.on("pageerror", (e) => errors.push(e.message));
        const fixture =
          state === "vault-form"
            ? "vault"
            : ["empty", "vault", "inactive-only"].includes(state)
              ? state
              : "list";
        await page.goto(
          `http://127.0.0.1:${port}/visual/equipment.html?theme=${theme}&state=${fixture}`,
        );
        await page.evaluate(() => document.fonts.ready);
        await page
          .getByRole("button", { name: /^Adicionar(?: equipamento)?$/ })
          .waitFor();
        assert.equal(
          await page.getByRole("button", { name: /^Adicionar/ }).count(),
          1,
          "one Add CTA across full shell and content",
        );
        if (state === "empty") {
          await page
            .getByRole("heading", {
              name: "Nenhum equipamento cadastrado",
              exact: true,
            })
            .waitFor();
          for (const selector of [
            "table",
            ".inspector",
            ".inventory-tools",
            ".list-footer",
            ".table-caption",
          ])
            assert.equal(
              await page.locator(selector).count(),
              0,
              `empty has no ${selector}`,
            );
          assert.equal(
            await page.locator(".empty-state").innerText(),
            "Nenhum equipamento cadastrado\nAdicionar equipamento",
          );
        }
        if (state === "inactive-only") {
          await page
            .getByRole("heading", {
              name: "Nenhum equipamento ativo",
              exact: true,
            })
            .waitFor();
          assert.equal(
            await page.getByLabel("Estado dos equipamentos").count(),
            1,
          );
          await page.getByRole("button", { name: "Mostrar inativos" }).click();
          await page
            .getByRole("button", { name: "Reativar", exact: true })
            .waitFor();
        }
        if (state === "no-results") {
          await page.getByLabel("Buscar equipamento").fill("no such fixture");
          await page
            .getByRole("heading", { name: "Nenhum resultado", exact: true })
            .waitFor();
          for (const selector of ["table", ".inspector", ".list-footer"])
            assert.equal(await page.locator(selector).count(), 0);
          await page.getByRole("button", { name: "Limpar busca" }).click();
          await page.getByRole("table").waitFor();
          await page.getByLabel("Buscar equipamento").fill("no such fixture");
          await page
            .getByRole("heading", { name: "Nenhum resultado", exact: true })
            .waitFor();
        }
        if (state === "vault") {
          assert.equal(
            await page
              .getByText(
                "Cofre indisponível. Confira a credencial do serviço nesta instalação.",
                { exact: true },
              )
              .count(),
            1,
          );
          assert.equal(
            await page.locator("table").count(),
            1,
            "inventory readable without vault",
          );
          assert.equal(
            await page
              .getByRole("button", { name: "Credenciais", exact: true })
              .isDisabled(),
            true,
          );
        }
        let expected = [];
        if (state === "vault-form") {
          await page
            .getByRole("button", { name: "Adicionar", exact: true })
            .click();
          const dialog = page.getByRole("dialog");
          await dialog
            .getByText(
              "Cofre indisponível. Confira a credencial do serviço nesta instalação.",
              { exact: true },
            )
            .waitFor();
          assert.equal(
            await page
              .getByText(
                "Cofre indisponível. Confira a credencial do serviço nesta instalação.",
                { exact: true },
              )
              .count(),
            1,
          );
          assert.equal(
            await dialog.getByLabel("Senha", { exact: true }).isDisabled(),
            true,
          );
          assert.equal(
            await dialog
              .getByRole("button", { name: "Salvar equipamento" })
              .isDisabled(),
            true,
          );
          expected = [
            { name: "Fechar painel" },
            { name: "Usar credencial existente" },
          ];
        }

        if (["device-form", "existing-form"].includes(state)) {
          await page
            .getByRole("button", { name: /^Adicionar(?: equipamento)?$/ })
            .click();
          const dialog = page.getByRole("dialog", {
            name: "Adicionar equipamento",
            exact: true,
          });
          await dialog.waitFor();
          for (const label of [
            "Nome",
            "IP de gerenciamento",
            "Usuário",
            "Senha",
            "Porta SSH",
          ])
            assert.equal(
              await dialog.getByLabel(label, { exact: true }).count(),
              1,
            );
          assert.equal(
            await dialog.getByLabel("Alias", { exact: true }).count(),
            0,
          );
          assert.equal(
            await dialog.getByLabel("Senha", { exact: true }).inputValue(),
            "",
          );
          assert.equal(await dialog.getByLabel("Porta SSH").inputValue(), "22");
          if (state === "existing-form") {
            await page
              .getByRole("button", { name: "Usar credencial existente" })
              .click();
            await page
              .getByText("Somente credenciais desta empresa.", { exact: true })
              .waitFor();
            assert.equal(
              await dialog.locator("input[type=password]").count(),
              0,
            );
          }
          expected = [
            { name: "Fechar painel" },
            { name: "Salvar equipamento" },
            {
              selector:
                state === "device-form" ? 'input[type="password"]' : "select",
            },
          ];
        }
        if (state === "credential-form") {
          await page
            .getByRole("button", { name: "Credenciais", exact: true })
            .click();
          await page.getByLabel("Credencial para editar").selectOption("read");
          expected = [
            { name: "Salvar credencial", disabled: true },
            { name: "Fechar painel" },
            { selector: 'input[type="password"]' },
          ];
        }
        if (state === "trust") {
          await page
            .getByRole("button", { name: "Verificar identidade", exact: true })
            .click();
          await page.getByRole("dialog").waitFor();
          expected = [
            { name: "Confirmar chave conferida", disabled: true },
            { name: "Fechar painel" },
            { selector: "form input" },
          ];
        }
        // Scroll each control into view and hit-test it. This is Chromium evidence only.
        const actionChecks = [];
        for (const control of expected) {
          const locator = control.selector
            ? page.locator(".dialog " + control.selector).first()
            : page.getByRole("button", { name: control.name, exact: true });
          await locator.scrollIntoViewIfNeeded();
          const measured = await page.evaluate(inspectActions, [control]);
          actionChecks.push({
            control,
            measurement: measured,
            stateAtMeasurement: "control scrolled into view",
          });
          assert.deepEqual(
            measured.failures,
            [],
            key + JSON.stringify(measured),
          );
        }
        const geometry = await page.evaluate(() => ({
          width: document.documentElement.scrollWidth,
          height: document.documentElement.scrollHeight,
          header: document.querySelector(".topbar").getBoundingClientRect()
            .height,
          zoom: getComputedStyle(document.querySelector(".app")).zoom,
          font: getComputedStyle(document.querySelector(".app")).fontFamily,
        }));
        assert.equal(geometry.width, width);
        assert.equal(geometry.height, height);
        assert.equal(geometry.header, 60);
        assert.equal(Number(geometry.zoom), 1);
        assert.match(geometry.font, /Roboto/);
        assert.deepEqual(errors, []);
        if (await page.getByRole("dialog").count())
          await page.locator(".dialog").evaluate((el) => {
            el.scrollTop = 0;
          });
        const capture = path.join(out, key + ".png");
        await page.screenshot({ path: capture });
        // Keep captured state intact until after screenshot; then exercise actual keyboard.
        if (await page.getByRole("dialog").count()) {
          const dialog = page.getByRole("dialog");
          await page
            .getByRole("button", { name: "Fechar painel", exact: true })
            .focus();
          await page.keyboard.press("Shift+Tab");
          assert.equal(
            await page
              .locator(":focus")
              .evaluate((el) => !!el.closest("[role=dialog]")),
            true,
          );
          await page.keyboard.press("Tab");
          assert.equal(
            await page.locator(":focus").getAttribute("aria-label"),
            "Fechar painel",
          );
          await page.keyboard.press("Escape");
          assert.equal(await dialog.count(), 0);
          const opener = [
            "device-form",
            "existing-form",
            "vault-form",
          ].includes(state)
            ? "Adicionar"
            : state === "credential-form"
              ? "Credenciais"
              : "Verificar identidade";
          assert.equal(
            await page.locator(":focus").innerText(),
            opener,
            "focus returned to the exact opener",
          );
        }
        const ref = await browser.newPage({
          viewport: { width, height },
          deviceScaleFactor: 1,
        });
        await ref.goto(
          pathToFileURL(referenceHTML).href +
            `#tela=4&tema=${theme === "dark" ? "escuro" : "claro"}&cenario=${state === "empty" ? "vazio" : "piloto"}`,
        );
        await ref.evaluate(() => document.fonts.ready);
        await ref.evaluate(
          ({ width, height }) => {
            const app = document.querySelector("#app");
            document.body.append(app);
            Object.assign(app.style, {
              position: "absolute",
              left: "0px",
              top: "-40px",
              width: width + "px",
              height: height + 40 + "px",
            });
            app.dataset.compact = width < 1250 ? "true" : "false";
            document.querySelector(".reviewbar").remove();
            document.querySelector("#viewport").remove();
          },
          { width, height },
        );
        const reference = path.join(out, key + "-reference.png");
        await ref.screenshot({ path: reference });
        rows.push({
          key,
          state,
          theme,
          viewport: [width, height],
          capture,
          reference,
          capture_sha256: sha(capture),
          reference_sha256: sha(reference),
          geometry,
          actionChecks,
          runtime: "Chromium isolated component fixture",
          reference_relation:
            "M04/DS-001 shell, tokens and controls; C02/C03 authorized content/CTA/dialog deltas. Original reference has no direct-enrollment modal.",
        });
        await ref.close();
        await page.close();
      }
  fs.writeFileSync(
    path.join(out, "capture-manifest.json"),
    JSON.stringify(
      {
        fixture: true,
        rows,
        inputs: [
          ...files("frontend/src"),
          ...files("frontend/visual"),
          ...files("frontend/public/fonts"),
          ...files(bundle),
          "frontend/wailsjs/go/presentation/App.js",
          "frontend/wailsjs/go/models.ts",
          "scripts/test-cofre-ux-visual.mjs",
          referenceHTML,
        ].map((p) => ({ path: p, sha256: sha(p) })),
      },
      null,
      2,
    ) + "\n",
  );
  console.log(
    `${rows.length} focal states; comparison review remains separate.`,
  );
} finally {
  await browser.close();
  await new Promise((resolve) => server.httpServer.close(resolve));
}
