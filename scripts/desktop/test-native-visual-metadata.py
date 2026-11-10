#!/usr/bin/env python3
from native_visual_metadata import native_metadata
sample={'key':'sample','viewport':[720,600],'bounds':{'width':720},'reachability':{'failures':[]},'errors':[]}
result=native_metadata(sample)
assert 'bounds' not in result and 'reachability' not in result and 'errors' not in result
assert result['browser_supplement']['reachability']['failures']==[]
assert result['browser_supplement']['bounds']==sample['bounds']
assert result['native_reachability'].startswith('NOT RUN') and sample['bounds']=={'width':720}
print('Browser/native measurement attribution: PASS; unmeasured native hit tests remain NOT RUN')
