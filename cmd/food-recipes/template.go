package main

import "html/template"

// ---------- Template (includes glyph palette) ----------

var indexTmpl = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Nirvana Recipe Finder</title>
<style>
:root{
  --mint-25:#daf7ee; --mint-50:#c6f2e6; --mint-100:#a2ecd9;
  --mint-200:#7de6cc; --mint-300:#58dfbf; --mint-400:#35d9b3;
  --mint-500:#22d8ad; --mint-600:#17b392; --mint-700:#118b73;
  --bg-dark-1:#0c2924; --bg-dark-2:#0e312b;
  --glass-tint:rgba(20,50,45,0.30);
  --text-900:#e9fffa; --text-700:#b6e6d9; --text-500:#8dd7c6;
}

/* Prefer locally-installed Glyphs-Mono for rendering buttons/inputs */
@font-face{
  font-family: "GlyphsMono";
  src: local("Glyphs-Mono"), local("Glyphs Mono");
  font-display: swap;
}
.glyphFont{
  font-family: "GlyphsMono","Glyphs-Mono","Glyphs Mono", ui-sans-serif, system-ui;
  letter-spacing: 0.02em;
}

*{box-sizing:border-box}
html,body{height:100%;margin:0;font-family:ui-sans-serif,system-ui,-apple-system,Segoe UI,Roboto,Helvetica,Arial}

body{
  color:var(--text-900);
  background:
    radial-gradient(900px 520px at 15% -10%, rgba(53,217,179,0.18), transparent 55%),
    radial-gradient(800px 480px at 110% 20%, rgba(88,223,191,0.14), transparent 50%),
    repeating-linear-gradient(45deg,
      rgba(53,217,179,0.08) 0px, rgba(53,217,179,0.08) 14px,
      rgba(17,139,115,0.10) 14px, rgba(17,139,115,0.10) 28px),
    linear-gradient(180deg, var(--bg-dark-1) 0%, var(--bg-dark-2) 100%);
}

.container{ min-height:100%; display:flex; align-items:center; justify-content:center; padding:24px; }

.card{
  width:min(1000px,92vw); position:relative;
  backdrop-filter: blur(26px) saturate(120%); -webkit-backdrop-filter: blur(26px) saturate(120%);
  background:linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.06)), var(--glass-tint);
  border:1px solid rgba(180,255,237,0.18); border-radius:24px; padding:28px;
  box-shadow:0 24px 60px rgba(12,41,36,0.55), inset 0 1px 0 rgba(255,255,255,0.06);
}

/* Header */
.header{display:flex;align-items:center;gap:14px;margin-bottom:12px}
.badge{
  background:linear-gradient(145deg,var(--mint-500),var(--mint-300));
  color:white;font-weight:700;border-radius:12px;padding:6px 10px;font-size:12px;
  box-shadow:0 8px 20px rgba(34,216,173,0.40);
}
h1{font-size:24px;margin:0}
.sub{color:var(--text-700);margin:6px 0 18px 0}

/* Tokenized input (search) */
.inputRow{position:relative; display:flex; gap:10px; flex-wrap:wrap}
.tokenBox{
  flex:1; min-height:50px; display:flex; align-items:center; flex-wrap:wrap; gap:8px;
  padding:8px 10px; border-radius:16px; border:1px solid rgba(255,255,255,0.10);
  background:linear-gradient(180deg, rgba(255,255,255,0.08), rgba(255,255,255,0.05));
}
.token{
  display:flex; align-items:center; gap:8px; padding:6px 10px; border-radius:999px;
  background:rgba(53,217,179,0.18); border:1px solid rgba(53,217,179,0.35); color:#dffef7;
  max-width:100%;
}
.token .text{white-space:nowrap; overflow:hidden; text-overflow:ellipsis; max-width:220px}
.token .x{ border:none; background:transparent; color:#eafff9; opacity:.85; cursor:pointer; font-weight:700; }
.tokenInput{ flex:1; min-width:160px; border:none; outline:none; background:transparent; color:var(--text-900); padding:8px 6px; font-size:16px; }
.tokenInput::placeholder{color:var(--text-500)}

button.primary{
  background:linear-gradient(180deg, var(--mint-500), var(--mint-600));
  color:white;font-weight:700;border:none;border-radius:14px;
  padding:12px 18px;cursor:pointer;
  box-shadow:0 14px 30px rgba(34,216,173,0.35);
  transition:transform .06s ease, box-shadow .2s ease, filter .2s, opacity .2s;
}
button.primary:hover{filter:saturate(110%); box-shadow:0 16px 36px rgba(34,216,173,0.45)}
button.primary:active{transform:translateY(1px); opacity:.95}

/* Autocomplete dropdown (more opacity + hidden scrollbar) */
.dropdown{
  position:absolute; left:0; right:180px; top:100%; z-index:20; margin-top:8px;
  border-radius:14px; overflow-y:auto; border:1px solid rgba(255,255,255,0.14);
  background:rgba(12,41,36,0.65);
  backdrop-filter: blur(22px) saturate(130%); -webkit-backdrop-filter: blur(22px) saturate(130%);
  box-shadow:0 20px 48px rgba(0,0,0,0.40);
  max-height:280px;
  scrollbar-width:none; -ms-overflow-style:none;
}
.dropdown::-webkit-scrollbar { display:none; }
.item{ padding:10px 12px; cursor:pointer; color:var(--text-900); border-bottom:1px solid rgba(255,255,255,0.06); }
.item:last-child{border-bottom:none}
.item:hover, .item.active{ background:rgba(53,217,179,0.18); }

/* Chips row + footer */
.aux{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-top:8px}
.chips{display:flex;gap:8px;flex-wrap:wrap}
.chip{ padding:6px 10px;border-radius:999px;font-size:12px; background:rgba(53,217,179,0.14); border:1px solid rgba(53,217,179,0.35); color:#dffef7 }
.footer{margin-top:16px;color:var(--text-700);font-size:12px;text-align:right}
kbd{ background:rgba(53,217,179,0.20); border-radius:6px; border:1px solid rgba(53,217,179,0.45); padding:2px 6px; color:#eafff9 }

/* Results */
.result{ margin-top:18px; border-radius:18px; padding:16px; background:linear-gradient(180deg, rgba(255,255,255,0.08), rgba(255,255,255,0.05)); border:1px solid rgba(255,255,255,0.10); }
.result h2{font-size:16px;margin:0 0 12px 0;color:#c9fff3}
.list{display:grid;grid-template-columns:1fr;gap:10px}
@media(min-width:720px){.list{grid-template-columns:1fr 1fr}}
.cardItem{ border-radius:16px;padding:12px 14px; background:linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.06)); border:1px solid rgba(255,255,255,0.10); box-shadow:0 6px 18px rgba(0,0,0,0.18); color:var(--text-900); }
.itemTitle{font-weight:700;margin-bottom:6px}
.itemMeta{color:var(--text-700);font-size:13px}
.warn{ color:#ffdede; background:rgba(255,61,61,0.12); border:1px solid rgba(255,61,61,0.25); padding:8px 10px; border-radius:10px; margin-top:10px; }

/* Floating Pill Dock */
.dock {
  position: fixed;
  left: 50%;
  bottom: max(16px, env(safe-area-inset-bottom));
  transform: translateX(-50%);
  display: flex; gap: 8px; padding: 10px; border-radius: 999px; z-index: 50;
  background: linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.06)), var(--glass-tint);
  border: 1px solid rgba(180,255,237,0.18);
  box-shadow: 0 18px 44px rgba(12,41,36,0.50), inset 0 1px 0 rgba(255,255,255,0.06);
  backdrop-filter: blur(22px) saturate(120%); -webkit-backdrop-filter: blur(22px) saturate(120%);
}
.dock-btn {
  appearance: none; border: 1px solid rgba(255,255,255,0.10);
  background: linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.05));
  color: var(--text-900); border-radius: 999px; padding: 10px 14px;
  display: flex; align-items: center; gap: 8px; font-size: 14px; font-weight: 600;
  cursor: pointer; transition: transform .06s ease, box-shadow .2s ease, background .2s ease, border-color .2s ease;
  box-shadow: 0 6px 16px rgba(0,0,0,0.20);
}
.dock-btn:hover { border-color: rgba(53,217,179,0.45); box-shadow: 0 10px 22px rgba(34,216,173,0.32); }
.dock-btn:active { transform: translateY(1px); }
.dock-btn.active {
  background: linear-gradient(180deg, rgba(34,216,173,0.22), rgba(34,216,173,0.12));
  border-color: rgba(53,217,179,0.55); box-shadow: 0 12px 26px rgba(34,216,173,0.40);
}
.dock-ico { width: 22px; height: 22px; border-radius: 999px; display: inline-grid; place-items: center; background: rgba(53,217,179,0.18); border: 1px solid rgba(53,217,179,0.35); font-size: 13px; }
@media (max-width: 520px) { .dock-btn .label { display: none; } .dock-btn { padding: 10px; } }

/* Glyphs Section */
.section{
  margin-top:22px; padding:16px; border-radius:18px;
  background:linear-gradient(180deg, rgba(255,255,255,0.08), rgba(255,255,255,0.05));
  border:1px solid rgba(255,255,255,0.10);
}
.section h2{font-size:18px; margin:0 0 10px 0; color:#c9fff3}
.formRow{display:flex; gap:10px; flex-wrap:wrap; align-items:flex-start}
.inputGlass, textarea.inputGlass{
  flex:1; min-width:200px; color:var(--text-900);
  border-radius:14px; border:1px solid rgba(255,255,255,0.10);
  background:linear-gradient(180deg, rgba(255,255,255,0.08), rgba(255,255,255,0.05));
  padding:12px 14px; font-size:14px; outline:none;
}
textarea.inputGlass{ min-height:70px; resize:vertical }
.inputGlass::placeholder{ color: var(--text-500) }
.help{ font-size:12px; color:var(--text-700) }

.glyphList{ display:grid; grid-template-columns:1fr; gap:10px; margin-top:10px }
@media(min-width:720px){ .glyphList{ grid-template-columns:1fr 1fr } }
.glyphCard{
  border-radius:16px; padding:12px 14px;
  background:linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.06));
  border:1px solid rgba(255,255,255,0.10); box-shadow:0 6px 18px rgba(0,0,0,0.18);
}
.glyphTitle{ font-weight:700; margin-bottom:6px }
.glyphSymbols{ font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; font-size:16px }
.glyphMeta{ color: var(--text-700); font-size:12px; margin-top:4px }
.gbtn{
  border:1px solid rgba(255,255,255,0.10);
  background:linear-gradient(180deg, rgba(255,255,255,0.10), rgba(255,255,255,0.05));
  color: var(--text-900); border-radius:10px; padding:6px 10px; cursor:pointer; font-size:12px;
}
.gbtn:hover{ border-color: rgba(53,217,179,0.45); }

/* Glyph palette (buttons rendering with custom font) */
.glyphPad{ display:inline-flex; flex-direction:column; gap:8px; margin-top:6px }
.glyphRow{ display:flex; gap:8px }
.glyphSpacer{ width:12px }
.glyphBtn{
  width:44px; height:44px; border-radius:12px;
  display:grid; place-items:center;
  font-size:22px; line-height:1; cursor:pointer; color:var(--text-900);
  border:1px solid rgba(255,255,255,0.12);
  background:linear-gradient(180deg, rgba(255,255,255,0.12), rgba(255,255,255,0.06));
  backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px);
  box-shadow:0 8px 18px rgba(0,0,0,0.25);
  transition:transform .06s ease, box-shadow .2s ease, border-color .2s ease, background .2s ease;
}
.glyphBtn:hover{ border-color: rgba(53,217,179,0.45); box-shadow:0 12px 28px rgba(34,216,173,0.38); }
.glyphBtn:active{ transform: translateY(1px) }
</style>
</head>
<body>
<div class="container">
  <div class="card">
    <div class="header">
      <span class="badge">Nirvana</span>
      <h1>Recipe Finder</h1>
    </div>
    <div class="sub">Type one or more ingredients. Press <strong>Enter</strong> to add; with the input empty, <strong>Enter</strong> searches.</div>

    <!-- Search input -->
    <div class="inputRow">
      <div class="tokenBox" id="tokenBox" aria-haspopup="listbox" aria-expanded="false">
        <div id="tokens"></div>
        <input id="ingInput" class="tokenInput" type="text" autocomplete="off" placeholder="Type an ingredient and press Enter…" />
      </div>
      <button class="primary" id="btn">Suggest</button>
      <div class="dropdown" id="dropdown" role="listbox" hidden></div>
    </div><br>

    <!-- Quick chips + tip -->
    <div class="aux">
      <div class="chips" id="chips"></div>
      <div class="footer">Tip: Enter = add, Enter again = search • ⌘/Ctrl+Enter = add & search</div>
    </div>

    <!-- Results -->
    <div class="result" id="result" style="display:none">
      <h2>Suggestions</h2>
      <div id="mapped" class="itemMeta"></div><br>
      <div id="unknown" class="warn" style="display:none"></div>
      <div class="list" id="list"></div>
    </div>

    <!-- Glyphs Section -->
    <div id="glyphsSec" class="section" style="margin-top:24px">
      <h2>Glyphs</h2>
      <div class="formRow" style="margin-bottom:10px">
        <input id="gName" class="inputGlass" type="text" maxlength="64" placeholder="Name (e.g., Sentinel Path)" />
        <input id="gSymbols" class="inputGlass glyphFont" type="text" maxlength="128" placeholder="Symbols (type or tap below)" />
      </div>

      <!-- Glyph palette -->
      <div class="glyphPad" id="glyphPad"></div>

      <div class="formRow" style="margin:8px 0">
        <textarea id="gDesc" class="inputGlass" maxlength="512" placeholder="Description (optional but recommended)"></textarea>
      </div>
      <div class="formRow" style="align-items:center">
        <button id="gSave" class="gbtn">Save Glyph</button>
        <span id="gMsg" class="help"></span>
      </div>

      <div class="glyphList" id="glyphList"></div>
    </div>
  </div>
</div>

<!-- Floating Dock -->
<nav class="dock" role="navigation" aria-label="Primary">
  <button class="dock-btn" data-nav="#home" aria-label="Home"><span class="dock-ico">🏠</span><span class="label">Home</span></button>
  <button class="dock-btn" data-nav="#ingredients" aria-label="Ingredients"><span class="dock-ico">🥗</span><span class="label">Ingredients</span></button>
  <button class="dock-btn" data-nav="#glyphs" aria-label="Glyphs"><span class="dock-ico">🔤</span><span class="label">Glyphs</span></button>
  <button class="dock-btn" data-nav="#settings" aria-label="Settings"><span class="dock-ico">⚙️</span><span class="label">Settings</span></button>
</nav>

<script>
/* ===== Ingredients search ===== */
let ALL_ING = [];
const tokens = [];

const el = (id) => document.getElementById(id);
const tokenBox = el('tokenBox');
const tokensWrap = el('tokens');
const input = el('ingInput');
const dropdown = el('dropdown');
const suggestBtn = el('btn');

function uniquePush(arr, v){ if(!arr.includes(v)) arr.push(v); }
function removeAt(arr, i){ arr.splice(i, 1); }
function renderTokens(){
  tokensWrap.innerHTML = '';
  tokens.forEach((t,i)=>{
    const d = document.createElement('div'); d.className='token';
    const span = document.createElement('span'); span.className='text'; span.textContent=t;
    const x = document.createElement('button'); x.className='x'; x.type='button'; x.setAttribute('aria-label', 'Remove'); x.textContent='×';
    x.onclick = () => { removeAt(tokens, i); renderTokens(); };
    d.appendChild(span); d.appendChild(x);
    tokensWrap.appendChild(d);
  });
  input.placeholder = tokens.length ? '' : 'Type an ingredient and press Enter…';
  tokenBox.setAttribute('aria-expanded', !dropdown.hidden ? 'true' : 'false');
}

/* Autocomplete */
let activeIndex = -1;
function filterSuggestions(q){
  const s = q.trim().toLowerCase();
  if(!s) return [];
  const cand = ALL_ING.filter(x => !tokens.includes(x));
  const pref = [], sub = [];
  cand.forEach(c=>{
    const lc = c.toLowerCase();
    if(lc.startsWith(s)) pref.push(c);
    else if(lc.includes(s)) sub.push(c);
  });
  return pref.concat(sub).slice(0, 50);
}
function renderDropdown(items){
  dropdown.innerHTML = '';
  if(items.length === 0){ dropdown.hidden = true; activeIndex = -1; return; }
  items.forEach((text, idx)=>{
    const it = document.createElement('div');
    it.className = 'item' + (idx===activeIndex ? ' active' : '');
    it.setAttribute('role','option');
    it.textContent = text;
    it.onclick = () => { addToken(text); };
    dropdown.appendChild(it);
  });
  dropdown.hidden = false;
}

/* Tokenization */
function addToken(text){
  const t = text.trim();
  if(!t) return;
  let final = t;
  const matches = filterSuggestions(t);
  if(matches.length && matches[0].toLowerCase() !== t.toLowerCase()){
    final = matches[0];
  }
  uniquePush(tokens, final);
  input.value = '';
  activeIndex = -1;
  renderTokens();
  renderDropdown([]); // hide
}
function currentSuggestions(){
  return Array.from(dropdown.querySelectorAll('.item')).map(n=>n.textContent);
}

/* Keyboard interactions */
input.addEventListener('keydown', (e)=>{
  const items = currentSuggestions();
  const commitKeys = ['Enter', 'Tab', ','];

  if (e.key === 'Escape') { dropdown.hidden = true; activeIndex = -1; return; }

  if (e.key === 'Backspace' && input.value.trim() === '' && tokens.length) {
    e.preventDefault(); tokens.pop(); renderTokens(); return;
  }

  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    const has = !dropdown.hidden && items.length > 0;
    if (!has) return;
    e.preventDefault();
    if (e.key === 'ArrowDown') activeIndex = (activeIndex + 1) % items.length;
    else activeIndex = (activeIndex - 1 + items.length) % items.length;
    renderDropdown(items);
    return;
  }

  if (commitKeys.includes(e.key)) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      if (input.value.trim() !== '') {
        if (!dropdown.hidden && items.length && activeIndex >= 0) addToken(items[activeIndex]);
        else addToken(input.value);
      }
      if (tokens.length) suggest();
      return;
    }
    if (e.key === 'Enter' && input.value.trim() === '') {
      e.preventDefault();
      if (tokens.length) suggest();
      return;
    }
    e.preventDefault();
    if (!dropdown.hidden && items.length && activeIndex >= 0) addToken(items[activeIndex]);
    else addToken(input.value);
  }
});

input.addEventListener('input', (e)=>{
  const q = e.target.value;
  const items = filterSuggestions(q);
  activeIndex = -1;
  renderDropdown(items);
});

tokenBox.addEventListener('keydown', (e)=>{
  if (e.key === 'Enter' && input.value.trim() === '' && tokens.length){
    e.preventDefault(); suggest();
  }
});

document.addEventListener('click', (e)=>{
  if(!tokenBox.contains(e.target) && !dropdown.contains(e.target)){
    dropdown.hidden = true; activeIndex = -1;
  }
});

async function fetchIngredients(){
  try{
    const r = await fetch('/api/ingredients');
    if(!r.ok) return [];
    return await r.json();
  }catch{ return []; }
}
function renderChips(all){
  const elc = document.getElementById('chips'); elc.innerHTML = '';
  all.slice(0, 10).forEach(name=>{
    const d = document.createElement('div'); d.className='chip'; d.textContent=name;
    d.onclick = ()=>{ addToken(name); input.focus(); };
    elc.appendChild(d);
  });
}

async function suggest(){
  if(tokens.length === 0) return;
  const have = encodeURIComponent(tokens.join(','));
  const r = await fetch('/api/suggest?have=' + have);
  const data = await r.json();

  document.getElementById('result').style.display = 'block';
  const mapped = (data.mapped||[]).join(', ') || '—';
  document.getElementById('mapped').textContent = 'Matched: ' + mapped;

  const unk = document.getElementById('unknown');
  if(data.unrecognized && data.unrecognized.length){
    unk.style.display='block';
    unk.textContent='Unrecognized: ' + data.unrecognized.join(', ');
  }else{
    unk.style.display='none';
  }

  const list = document.getElementById('list'); list.innerHTML='';
  (data.suggestions||[]).forEach(rec=>{
    const item = document.createElement('div'); item.className='cardItem';
    const t = document.createElement('div'); t.className='itemTitle';
    t.textContent = rec.inputs.join(' + ') + ' \u2192 ' + rec.output + ' (x' + rec.qty + ')';
    const m = document.createElement('div'); m.className='itemMeta';
    m.textContent = 'Inputs: ' + rec.inputs.join(', ');
    item.appendChild(t); item.appendChild(m); list.appendChild(item);
  });
}
suggestBtn.onclick = suggest;
tokenBox.addEventListener('click', ()=> input.focus());

/* ===== Glyphs UI ===== */
const gName = el('gName');
const gSymbols = el('gSymbols');
const gDesc = el('gDesc');
const gSave = el('gSave');
const gMsg = el('gMsg');
const gList = el('glyphList');

function msg(text, ok){
  gMsg.textContent = text || '';
  gMsg.className = ok ? 'help success' : (text ? 'help err' : 'help');
}
function glyphCard(g){
  const d = document.createElement('div'); d.className='glyphCard';
  const title = document.createElement('div'); title.className='glyphTitle'; title.textContent = g.name;
  const sym = document.createElement('div'); sym.className='glyphSymbols glyphFont'; sym.textContent = g.symbols;
  const meta = document.createElement('div'); meta.className='glyphMeta';
  const created = new Date(g.created_at);
  meta.textContent = 'Saved ' + created.toLocaleString() + (g.description ? ' • ' + g.description : '');
  const row = document.createElement('div'); row.style.marginTop = '8px';
  const copy = document.createElement('button'); copy.className='gbtn'; copy.textContent='Copy Symbols';
  copy.onclick = async ()=>{ try{ await navigator.clipboard.writeText(g.symbols); msg('Copied to clipboard', true); }catch{ msg('Copy failed', false); } };
  row.appendChild(copy);
  d.appendChild(title); d.appendChild(sym); d.appendChild(meta); d.appendChild(row);
  return d;
}
async function loadGlyphs(){
  try{
    const r = await fetch('/api/glyphs');
    if(!r.ok) throw new Error('load failed');
    const arr = await r.json();
    gList.innerHTML = '';
    (arr||[]).forEach(g => gList.appendChild(glyphCard(g)));
  }catch(e){
    msg('Failed to load glyphs', false);
  }
}
async function saveGlyph(){
  msg('', true);
  const name = gName.value.trim();
  const symbols = gSymbols.value.trim();
  const description = gDesc.value.trim();
  if(!name){ msg('Name is required', false); gName.focus(); return; }
  if(!symbols){ msg('Symbols are required', false); gSymbols.focus(); return; }
  try{
    const r = await fetch('/api/glyphs', {
      method:'POST',
      headers:{'Content-Type':'application/json'},
      body: JSON.stringify({name, symbols, description})
    });
    if(!r.ok){
      const txt = await r.text();
      throw new Error(txt || 'save failed');
    }
    gName.value = ''; gSymbols.value=''; gDesc.value='';
    await loadGlyphs();
    msg('Glyph saved', true);
  }catch(e){
    msg(e.message || 'Save failed', false);
  }
}

/* ===== Glyph Palette ===== */
const GLYPH_ROWS = [
  "ABC",
  "DEF",
  "1234567890"
];

function insertGlyph(ch){
  const inp = document.getElementById('gSymbols');
  const start = inp.selectionStart ?? inp.value.length;
  const end   = inp.selectionEnd ?? start;
  const before = inp.value.slice(0, start);
  const after  = inp.value.slice(end);
  inp.value = before + ch + after;
  const pos = start + ch.length;
  try { inp.setSelectionRange(pos, pos); } catch {}
  inp.focus();
}

function renderGlyphPad(){
  const pad = document.getElementById('glyphPad');
  pad.innerHTML = '';
  GLYPH_ROWS.forEach(row => {
    const rowEl = document.createElement('div');
    rowEl.className = 'glyphRow';
    Array.from(row).forEach(ch => {
      if (ch === ' ') {
        const sp = document.createElement('div'); sp.className = 'glyphSpacer'; rowEl.appendChild(sp); return;
      }
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'glyphBtn glyphFont';
      b.textContent = ch;
      b.title = ch;
      b.addEventListener('click', () => insertGlyph(ch));
      rowEl.appendChild(b);
    });
    pad.appendChild(rowEl);
  });
}

/* ===== Dock behavior ===== */
const dock = document.querySelector('.dock');
const dockBtns = Array.from(dock.querySelectorAll('.dock-btn'));
function setActiveByHash(h) { dockBtns.forEach(b => b.classList.toggle('active', b.getAttribute('data-nav') === h)); }
function navigateTo(h) {
  setActiveByHash(h);
  if (location.hash !== h) location.hash = h;
  const map = {
    '#home': document.querySelector('.card'),
    '#ingredients': document.querySelector('.inputRow'),
    '#glyphs': document.getElementById('glyphsSec'),
    '#settings': document.querySelector('.footer')
  };
  const target = map[h];
  if (target) { try { target.scrollIntoView({ behavior: 'smooth', block: 'center' }); } catch { target.scrollIntoView(); } }
}
dockBtns.forEach(btn => btn.addEventListener('click', () => navigateTo(btn.getAttribute('data-nav'))));
setActiveByHash(location.hash || '#home');
window.addEventListener('hashchange', () => setActiveByHash(location.hash));

/* ===== Init ===== */
fetchIngredients().then(arr => { ALL_ING = arr || []; renderChips(ALL_ING); });
renderTokens();
loadGlyphs();
renderGlyphPad();
gSave.onclick = saveGlyph;
</script>
</body>
</html>`))
