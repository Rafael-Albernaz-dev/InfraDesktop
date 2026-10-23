import { test } from "node:test";
import assert from "node:assert/strict";
import { runChecked, assertRendering } from "./visual-checks.mjs";

test("both visual subprocesses reject missing executables, nonzero and signals", () => {
  for (const label of ["render investigation", "comparison"]) {
    assert.throws(
      () => runChecked(label, "/nonexistent/id-visual", [], { stdio: "pipe" }),
      /ENOENT/,
    );
    assert.throws(
      () =>
        runChecked(label, process.execPath, ["-e", "process.exit(23)"], {
          stdio: "pipe",
        }),
      /exit 23/,
    );
    assert.throws(
      () =>
        runChecked(
          label,
          process.execPath,
          ["-e", "process.kill(process.pid, 'SIGTERM')"],
          { stdio: "pipe" },
        ),
      /SIGTERM/,
    );
    runChecked(label, process.execPath, ["-e", "process.exit(0)"], {
      stdio: "pipe",
    });
  }
});
test("geometry/font/token assertions reject wrong inspector, sidebar, toolbar, font and color", () => {
  for (const width of [1440,1100,1088,900,768,720]) {
    const bounds = {
      width,
      height: 860,
      zoom: 1,
      sidebar: width<1000?0:width < 1250 ? 200 : 224,
      header: 60,
      inspector: width<1100 ? width-(width<1000?0:200) : width < 1250 ? 268 : 300,
      font: "Roboto, Arial",
      fontSize: "15px",
      textColor: "rgb(32, 33, 36)",
      fonts: [{ family: "Roboto", status: "loaded" }],
    };
    const state = { width, height: 860, scene: 4, theme: "light", inspectorExpected: true };
    assertRendering(bounds, state, []);
    for (const [key, value] of Object.entries({
      inspector: 280,
      missingInspector: undefined,
      sidebar: 210,
      header: 55,
      font: "Arial",
      fontSize: "12px",
      textColor: "red",
      zoom: 2,
      fonts: [],
    })) {
      assert.throws(
        () => assertRendering({ ...bounds, [key === "missingInspector" ? "inspector" : key]: value }, state, []),
        /Unexpected rendering/,
      );
    }
  }
});

test("legitimate absent inspector requires explicit expectation", () => {
 const bounds = {width:1440,height:860,zoom:1,sidebar:224,header:60,font:"Roboto",fontSize:"15px",textColor:"rgb(32, 33, 36)",fonts:[{family:"Roboto",status:"loaded"}]};
 const state = {width:1440,height:860,scene:4,theme:"light",inspectorExpected:false};
 assertRendering(bounds,state,[]);
 assert.throws(()=>assertRendering({...bounds,inspector:300},state,[]), /Unexpected rendering/);
 assert.throws(()=>assertRendering(bounds,{...state,inspectorExpected:undefined},[]), /Unexpected rendering/);
});
