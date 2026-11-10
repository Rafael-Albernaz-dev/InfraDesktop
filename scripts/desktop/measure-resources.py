"""Local optimized process measurements. No persistent sampler or human processes."""
from pathlib import Path
import os,time,platform,hashlib
HZ=os.sysconf('SC_CLK_TCK')
def tree(pid):
 result=[pid]
 try:
  for child in Path(f'/proc/{pid}/task/{pid}/children').read_text().split():result+=tree(int(child))
 except FileNotFoundError:pass
 return result

def snapshot(ui,daemon):
 rows=[]
 for pid in sorted(set(tree(ui)+tree(daemon))):
  try:
   p=Path('/proc')/str(pid);stat=p.joinpath('stat').read_text().rsplit(')',1)[1].split();mem={}
   for line in p.joinpath('smaps_rollup').read_text().splitlines()[1:]:
    parts=line.split()
    if len(parts)>1:mem[parts[0].strip(':')]=int(parts[1])
   rows.append({'pid':pid,'name':p.joinpath('comm').read_text().strip(),'RSS_KiB':mem['Rss'],'PSS_KiB':mem['Pss'],'private_KiB':mem.get('Private_Clean',0)+mem.get('Private_Dirty',0),'cpu_ticks':int(stat[11])+int(stat[12])})
  except FileNotFoundError:pass
 return {'at_monotonic':time.monotonic(),'process_count':len(rows),'RSS_sum_KiB':sum(r['RSS_KiB'] for r in rows),'PSS_sum_KiB':sum(r['PSS_KiB'] for r in rows),'private_sum_KiB':sum(r['private_KiB'] for r in rows),'processes':rows}
def delta(a,b):
 before={r['pid']:r['cpu_ticks'] for r in a['processes']};elapsed=b['at_monotonic']-a['at_monotonic'];ticks=sum(r['cpu_ticks']-before.get(r['pid'],r['cpu_ticks']) for r in b['processes'])
 return {'elapsed_seconds':elapsed,'cpu_one_core_percent':100*ticks/HZ/elapsed,'cpu_ticks':ticks,'processes_before':a['process_count'],'processes_after':b['process_count']}
def hardware():
 cpu=next(l.split(':',1)[1].strip() for l in Path('/proc/cpuinfo').read_text().splitlines() if l.startswith('model name'))
 return {'system':platform.platform(),'CPU':cpu,'logical_cpus':os.cpu_count(),'MemTotal':Path('/proc/meminfo').read_text().splitlines()[0],'loadavg':os.getloadavg(),'method':'/proc stat CPU ticks; smaps_rollup RSS, PSS and private pages for owned UI descendants plus daemon. RSS sums count shared pages repeatedly; PSS apportions them. CPU 100% = one logical core. AT-SPI navigation timings include automation overhead. Xvfb software renderer on this workstation, not 4 GB certification.'}
def binary_hashes(root):return {str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest() for p in (root/'build/bin').glob('infradesktop*') if p.is_file()}
