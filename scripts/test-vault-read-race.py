"""Deterministically schedule chmod between DEV authorization and stable read.
The overlay injects only the competing filesystem operation, never replaces
ReadKey, stat, read, identity, or authorization checks. No key bytes are logged.
"""
import json, pathlib, subprocess, tempfile
root=pathlib.Path(__file__).resolve().parents[1]
source=root/'internal/adapters/vault/provider_linux.go'
tests=root/'internal/adapters/vault/provider_linux_test.go'
with tempfile.TemporaryDirectory(prefix='id-dev-read-race-') as tmp:
    tmp=pathlib.Path(tmp)
    print('synthetic overlay root:', tmp, '; cleanup: TemporaryDirectory after go test',flush=True)
    text=source.read_text()
    anchor='\treturn stableKey(f, path'
    if text.count(anchor)!=1: raise SystemExit('DEV scheduling anchor changed')
    text=text.replace(anchor, '\tif e := os.Chmod(path, 0660); e != nil { return nil, e }\n'+anchor)
    (tmp/'provider.go').write_text(text)
    (tmp/'provider_test.go').write_text(tests.read_text()+'''
func TestDEVAuthorizationSnapshotRace(t *testing.T) {
 p := filepath.Join(t.TempDir(), "synthetic-key")
 if e := os.WriteFile(p, bytes.Repeat([]byte{23},32),0600); e != nil { t.Fatal(e) }
 if b,e := ReadKey(p); e == nil || b != nil { t.Fatal("DEV accepted key exposed between authorized stat and read") }
}
''')
    overlay={'Replace':{str(source):str(tmp/'provider.go'),str(tests):str(tmp/'provider_test.go')}}
    (tmp/'overlay.json').write_text(json.dumps(overlay))
    argv=['go','test','-race','-count=1','-overlay',str(tmp/'overlay.json'),'./internal/adapters/vault','-run','^TestDEVAuthorizationSnapshotRace$','-v']
    print('injection: os.Chmod(path, 0660) immediately after DEV authorization',flush=True)
    raise SystemExit(subprocess.run(argv,cwd=root).returncode)
