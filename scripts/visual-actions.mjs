// Runs unchanged in the actual browser DOM. Explicit expectations catch removed,
// hidden or incorrectly disabled actions; geometry catches ancestor clipping.
export function inspectActions(expected) {
  const container=document.querySelector('[role="dialog"]')||document.querySelector('.account-dropdown')||document.querySelector('.navigation-open .sidebar')||document.querySelector('.app');
  const all=[...container.querySelectorAll('button,input:not([type="hidden"]),textarea,select,[tabindex="0"]')];
  const name=el=>el.getAttribute('aria-label')||el.textContent.trim().replace(/\s+/g,' ');
  const failures=[];const required=[];
  for(const item of expected) {
    const found=item.selector?container.querySelector(item.selector):all.find(el=>name(el)===item.name);
    if(!found){failures.push('missing '+(item.name||item.selector));continue;}
    required.push(found);
    if(found.matches(':disabled')!==Boolean(item.disabled))failures.push('disabled state '+(item.name||item.selector));
    if(!found.getClientRects().length||getComputedStyle(found).visibility!=='visible'||found.closest('[hidden]'))failures.push('hidden '+(item.name||item.selector));
  }
  const scrollers=[...document.querySelectorAll('*')].filter(e=>e.scrollHeight>e.clientHeight||e.scrollWidth>e.clientWidth).map(e=>[e,e.scrollLeft,e.scrollTop]);
  let checked=0;
  for(const el of new Set([...all.filter(el=>!el.matches(':disabled')),...required])) {
    if(!el.getClientRects().length||el.closest('[hidden]'))continue;
    if(Number(getComputedStyle(el).opacity)===0)failures.push('transparent '+name(el));
    const own=getComputedStyle(el);if((own.zoom&&Number(own.zoom)!==1)||own.transform!=='none')failures.push('scaled control '+name(el));
    const ancestors=[];for(let p=el.parentElement;p;p=p.parentElement)ancestors.push([p,p.scrollLeft,p.scrollTop]);
    el.scrollIntoView({block:'nearest',inline:'nearest'});
    for(const [p,x,y] of ancestors){const css=getComputedStyle(p);if((css.overflowX==='hidden'&&p.scrollLeft!==x)||(css.overflowY==='hidden'&&p.scrollTop!==y))failures.push('programmatic-only scroll '+name(el));}
    const r=el.getBoundingClientRect();if(!r.width||!r.height){failures.push('empty '+name(el));continue;}
    checked++;let left=0,top=0,right=innerWidth,bottom=innerHeight;
    for(let parent=el.parentElement;parent;parent=parent.parentElement){
      const css=getComputedStyle(parent),p=parent.getBoundingClientRect();
      if(Number(css.opacity)===0||css.visibility!=='visible'||(css.zoom&&Number(css.zoom)!==1)||css.transform!=='none')failures.push('invisible/scaled ancestor '+name(el));
      if(/hidden|clip|auto|scroll/.test(css.overflowX)){left=Math.max(left,p.left+parent.clientLeft);right=Math.min(right,p.left+parent.clientLeft+parent.clientWidth);}
      if(/hidden|clip|auto|scroll/.test(css.overflowY)){top=Math.max(top,p.top+parent.clientTop);bottom=Math.min(bottom,p.top+parent.clientTop+parent.clientHeight);}
    }
    if(r.left<left-1||r.right>right+1||r.top<top-1||r.bottom>bottom+1){failures.push('clipped '+name(el));continue;}
    const inset=Math.min(6,r.width/4,r.height/4);
    const points=[[r.x+r.width/2,r.y+r.height/2],[r.left+inset,r.top+inset],[r.right-inset,r.top+inset],[r.left+inset,r.bottom-inset],[r.right-inset,r.bottom-inset]];
    if(points.some(([x,y])=>{const hit=document.elementFromPoint(x,y);return !hit||!(el===hit||el.contains(hit));}))failures.push('occluded '+name(el));
  }
  for(const [el,x,y] of scrollers){el.scrollLeft=x;el.scrollTop=y;}
  return {checked,expected:expected.length,failures};
}
export function expectedActions(state,width) {
  const names=(...items)=>items.map(name=>({name}));
  if(state.variant==='navigation')return [...names('Selecionar empresa','Dashboard','Equipamentos','Topologia','Configurações','Aparência','Segurança'),...(width<1000?names('Fechar navegação'):[])];
  if(state.variant==='menu')return names('Claro','Escuro','Segurança','Sair');
  if(state.variant?.startsWith('security'))return [...names('Claro','Escuro'),...(state.variant==='security-code'?[{selector:'#new-recovery'},...names('Copiar código','Salvar .txt','Guardei meu código')]:[{selector:'#security-password'},...names(state.variant==='security-unknown'?'Gerar outro código novo':'Gerar novo código')])];
  if(state.scene===1)return [...names('Claro','Escuro'),...(state.variant==='code'?[{selector:'textarea[aria-label="Código de recuperação"]'},...names('Copiar código','Salvar .txt','Guardei meu código')]:[{selector:'#access-user'},{selector:'#access-password'},...(state.variant==='recover'?[{selector:'#recovery-input'}]:[]),{name:state.variant==='register'?'Criar conta local':state.variant==='recover'?'Redefinir senha':'Entrar',disabled:state.variant==='unavailable'},...names(state.variant==='register'||state.variant==='recover'?'Voltar ao login':'Recuperar acesso')])];
  const global=[...names('Abrir configurações','Conta: Operador'),{name:'Notificações indisponíveis',disabled:true},...(width<1000?names('Navegação'):names('Selecionar empresa'))];
  if(state.scene===2)return [...global,...names('Nova empresa','Abrir empresa'),{selector:'.company-row[aria-selected="true"]'}];
  if(state.scene===7)return [...global,...names('Claro','Escuro'),...(state.variant==='no-company'?[]:[...names('Escolher logo','Restaurar padrão'),{selector:'#brand-name'},...names('Turquesa','Violeta','Cobre','Azul','Verde','Rosa','Índigo','Ardósia'),{name:'Salvar',disabled:true},{name:'Cancelar',disabled:true}])];
  if(state.scene===4&&!state.empty)return [...global,{selector:'[aria-label="Buscar equipamento"]'},{selector:'[aria-label="Estado dos equipamentos"]'}];
  return global;
}
