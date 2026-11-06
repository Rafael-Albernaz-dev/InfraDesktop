import {test} from 'node:test';
import assert from 'node:assert/strict';
import {Linter} from 'eslint';
import ts from 'typescript-eslint';
import {plugin,rulesFor} from './eslint-quality.mjs';
test('G2 counts React hook import aliases',()=>{
 const lint=code=>new Linter().verify(code,[{files:['**/*.tsx'],languageOptions:{parser:ts.parser},plugins:{quality:plugin},rules:rulesFor('fixture.tsx')}],{filename:'fixture.tsx'});
 const states=n=>"import {useState as useField} from 'react';function View(){"+'useField(0);'.repeat(n)+'}';
 assert(!lint(states(8)).some(m=>m.ruleId==='quality/state-count'));
 assert(lint(states(9)).some(m=>m.ruleId==='quality/state-count'));
 const refs="import {useRef as useOwner} from 'react';function View(){"+'useOwner(0);'.repeat(7)+'}';
 assert(lint(refs).some(m=>m.ruleId==='quality/ref-count'));
});
