import {test} from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {ESLint,Linter} from 'eslint';
import ts from 'typescript-eslint';
import {plugin,rulesFor} from './eslint-quality.mjs';
test('effective new source configs enforce G1 and G4 in src and visual',async()=>{
 const lint=new ESLint();
 for(const path of ['src/new-quality-fixture.tsx','visual/new-quality-fixture.tsx']){
  const {rules}=await lint.calculateConfigForFile(path);
  for(const [rule,max] of Object.entries({'max-lines-per-function':60,'max-lines':500})){
   assert.equal(rules[rule][0],2);assert.equal(rules[rule][1].max,max);
   assert.equal(rules[rule][1].skipBlankLines,true);assert.equal(rules[rule][1].skipComments,true);
  }
  for(const [rule,max] of Object.entries({complexity:15,'max-params':5,'max-depth':4})){
   assert.equal(rules[rule][0],2);assert.equal(rules[rule][1],max);
  }
  for(const rule of ['no-nested-ternary','react-hooks/exhaustive-deps','react-refresh/only-export-components'])assert.equal(rules[rule][0],2);
 }
 assert.equal((await lint.calculateConfigForFile('src/new-quality-fixture.test.tsx')).rules['max-lines-per-function'][1].max,120);
 assert.match(fs.readFileSync('../Makefile','utf8'),/eslint \. --max-warnings 0 --no-inline-config/);
});
test('G2 accepts its boundary and rejects the seventh ref',()=>{
 const config=[{files:['**/*.tsx'],languageOptions:{parser:ts.parser},plugins:{quality:plugin},rules:rulesFor('fixture.tsx')}];
 const run=n=>new Linter().verify('function f(){'+('useRef(0);'.repeat(n))+'useState(0);'.repeat(8)+'}',config,{filename:'fixture.tsx'});
 assert(!run(6).some(m=>m.ruleId.startsWith('quality/')));
 assert(run(7).some(m=>m.ruleId==='quality/ref-count'));
});
