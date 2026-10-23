#!/usr/bin/env python3
"""Explicit isolated Wails presentation fixture; auth uses the real product gate."""
from pathlib import Path
import hashlib
import importlib.util
import json
import os
import shutil
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location('native', Path(__file__).with_name('test-desktop.py'))
native = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(native)

# Compiled only through a test overlay; normal main.go/bindings stay intact.
WRAPPER = r'''
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"io/fs"
	"os"
	"strings"
	"testing/fstest"
)

//go:embed all:build/ui-021-wails-fixture/dist
var fixture embed.FS

//go:embed packaging/linux/assets/infradesktop-512.png
var icon []byte

func main() {
	content := fstest.MapFS{}
	base, _ := fs.Sub(fixture, "build/ui-021-wails-fixture/dist")
	_ = fs.WalkDir(base, ".", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			b, e := fs.ReadFile(base, p)
			if e != nil {
				return e
			}
			content[p] = &fstest.MapFile{Data: b, Mode: 0444}
		}
		return nil
	})
	query, _ := json.Marshal("?" + os.Getenv("INFRADESKTOP_FIXTURE_QUERY"))
	index := content["index.html"]
	index.Data = []byte(strings.Replace(string(index.Data), "<head>", "<head><script>history.replaceState(null,'',"+string(query)+")</script>", 1))
	if e := wails.Run(&options.App{Title: "InfraDesktop — ISOLATED PRESENTATION FIXTURE", Width: 1440, Height: 860, MinWidth: 720, MinHeight: 600, Linux: &linux.Options{Icon: icon}, AssetServer: &assetserver.Options{Assets: content}}); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
'''


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def main():
    source = os.environ.get('INFRADESKTOP_VISUAL_CAPTURE')
    if not source:
        source = (ROOT / 'build/ui-021-visual/latest.txt').read_text().strip()
    manifest = json.loads((Path(source) / 'capture-manifest.json').read_text())
    states = manifest['rows']
    if any(not row.get('reference') for row in states):
        print('Wails fidelity NOT RUN: private references absent; no comparison or review produced.')
        return
    stamp = time.strftime('%Y%m%dT%H%M%S')
    work = ROOT / 'build/ui-021-wails-fixture' / stamp
    work.mkdir(parents=True, exist_ok=False)
    bundle = Path(source) / 'fixture-bundle'
    shutil.copytree(bundle, work / 'dist')
    shutil.copy2(bundle / 'visual/index.html', work / 'dist/index.html')
    embed_path = str(work.relative_to(ROOT) / 'dist')
    (work / 'main.go.txt').write_text(WRAPPER.replace('build/ui-021-wails-fixture/dist', embed_path))
    (work / 'overlay.json').write_text(json.dumps({'Replace': {str(ROOT / 'main.go'): str(work / 'main.go.txt')}}))
    env = dict(os.environ, PATH=str(ROOT / '.tools/go1.27.3/go/bin') + ':' + str(ROOT / '.tools/bin') + ':' + os.environ['PATH'],
               GOTOOLCHAIN='local', GOPROXY='off', GOSUMDB='off', GOFLAGS='-mod=readonly')
    binary = work / 'infradesktop-visual'
    subprocess.run(['go', 'build', '-trimpath', '-tags', 'webkit2_41,production', '-overlay',
                    str(work / 'overlay.json'), '-o', str(binary), '.'], env=env, check=True)
    out = ROOT / 'build/ui-021-wails-visual' / stamp
    out.mkdir(parents=True, exist_ok=False)
    rows = []
    with tempfile.TemporaryDirectory(prefix='id-wails-visual-') as tmp:
        for row in states:
            query = f'scene={row["scene"]}&theme={row["theme"]}'
            if row.get('variant'):
                query += '&variant='+row['variant']
            if row.get('empty'):
                query += '&empty=1'
            if row.get('alt'):
                query += '&alt=1'
            child = subprocess.Popen([str(binary)], env=dict(
                env, INFRADESKTOP_FIXTURE_QUERY=query, INFRADESKTOP_RUNTIME=tmp,
                INFRADESKTOP_CLIENT_STATE=tmp, NO_AT_BRIDGE='0', GTK_MODULES='atk-bridge', GDK_BACKEND='x11',GDK_SCALE='1',GDK_DPI_SCALE='1'),
                stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
            try:
                app = native.wait_for('owned isolated fixture application', lambda: native.owned_app(child))
                expected_status='Reconectando ao serviço' if row.get('variant')=='unavailable' else 'Serviço ativo'
                if row.get('variant','').startswith('security'):
                    native.wait_for('security fixture dialog rendered',lambda:native.find(app,'dialog','Segurança'))
                else:
                    native.wait_for('fixture rendered', lambda: expected_status in native.contents(app))
                native.configure_window(child, *row['viewport'])
                if row.get('variant')=='menu':native.click(app,'Conta: Operador')
                if row.get('variant')=='navigation':
                    if row['viewport'][0]<1000:native.click(app,'Navegação')
                    native.click(app,'Configurações')
                target = out / (row['key'] + '.png')
                if row['scene'] == 1:
                    label = 'Escuro' if row['theme'] == 'dark' else 'Claro'
                    button = native.wait_for('access theme focus', lambda:
                        native.find(app, 'toggle button', label) or native.find(app, 'push button', label))
                    native.type_label_x11(child, button, '', activate=True, keyboard_focus=True)
                    native.wait_for('actual access keyboard focus',lambda:button.get_state_set().contains(native.Atspi.StateType.FOCUSED))
                # Fixtures start without a field focus; preserve state and avoid clicking the narrow navigation trigger.
                previous=[None];measured=[None]
                def stable_frame():
                    measured[0]=native.capture(child,target,preserve_focus=True)
                    current=sha(target);stable=current==previous[0];previous[0]=current
                    return stable
                native.wait_for('two consecutive painted frames after resize/actions',stable_frame)
                geometry=measured[0]
                assert geometry['size']==row['viewport'], 'native viewport differs from named state'
                rows.append({**row, 'capture': str(target), 'capture_sha256': sha(target),
                             'native_geometry':geometry,'display':os.environ.get('DISPLAY'),'DPR':1,'scale':1,'decoration':'Xvfb client without WM frame','runtime': 'Wails WebKitGTK, isolated presentation fixture'})
                if row.get('variant')=='navigation' and row['viewport'][0]<1000:
                    native.click(app,'Fechar navegação')
                    native.wait_for('native navigation close returns focus',lambda:native.find(app,'push button','Navegação').get_state_set().contains(native.Atspi.StateType.FOCUSED))
                native.close_window(child, app)
            finally:
                native.terminate_ui(child)
    inputs = manifest['inputs'] + [
        {'path': str(path), 'sha256': sha(path)} for path in (
            ROOT / 'scripts/test-desktop.py', ROOT / 'scripts/test-process.py', binary,
            work / 'main.go.txt', Path(__file__).resolve())
    ]
    (out / 'capture-manifest.json').write_text(json.dumps({
        'fixture': True, 'auth': 'NOT PROVEN HERE — separate real product journey',
        'runtime': 'Optimized Go/Wails GTK/WebKit 2.52.6, X11/Xvfb, DPR1, 100% zoom',
        'inputs': inputs, 'rows': rows,
    }, indent=2) + '\n')
    subprocess.run(['python3', 'scripts/compare-visual.py', '--captures', str(out)], check=True)
    (ROOT / 'build/ui-021-wails-visual/latest.txt').write_text(str(out) + '\n')
    print(f'{len(rows)} isolated Wails states captured/compared; auth uses separate real journey. {out}')


if __name__ == '__main__':
    main()
