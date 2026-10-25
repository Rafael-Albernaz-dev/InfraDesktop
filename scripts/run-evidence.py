import subprocess,time,json,sys,pathlib,datetime
name=sys.argv[1];argv=sys.argv[2:];p=pathlib.Path('docs/evidence/bf01');p.mkdir(exist_ok=True)
a=time.monotonic();start=datetime.datetime.now(datetime.timezone.utc).isoformat()
with (p/(name+'.log')).open('w') as out:r=subprocess.run(argv,stdout=out,stderr=subprocess.STDOUT)
d={'name':name,'argv':argv,'started_utc':start,'seconds':round(time.monotonic()-a,3),'exit':r.returncode}
(p/(name+'.json')).write_text(json.dumps(d,indent=2));print(json.dumps(d));print((p/(name+'.log')).read_text()[-6500:]);sys.exit(r.returncode)
