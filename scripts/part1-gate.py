#!/usr/bin/env python3
"""Run one authorized gate, keep exit/duration/log in this delivery only."""
import datetime,json,pathlib,subprocess,sys,time
name=sys.argv[1];argv=sys.argv[2:];out=pathlib.Path('docs/evidence/l04-l05-parte1');out.mkdir(exist_ok=True)
if (out/(name+'.json')).exists():
 index=1
 while (out/(name+'-attempt'+str(index)+'.json')).exists():index+=1
 for suffix in ('.json','.log'):
  source=out/(name+suffix)
  if source.exists():source.rename(out/(name+'-attempt'+str(index)+suffix))
start=time.monotonic();at=datetime.datetime.now(datetime.timezone.utc).isoformat()
with (out/(name+'.log')).open('w') as log:result=subprocess.run(argv,stdout=log,stderr=subprocess.STDOUT)
record={'name':name,'argv':argv,'started_utc':at,'seconds':round(time.monotonic()-start,3),'exit':result.returncode}
(out/(name+'.json')).write_text(json.dumps(record,indent=2)+'\n');print(json.dumps(record));print((out/(name+'.log')).read_text()[-9000:]);sys.exit(result.returncode)
