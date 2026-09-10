'use strict';
const $ = id => document.getElementById(id);
const icon = name => { const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg'); const use = document.createElementNS(svg.namespaceURI, 'use'); use.setAttribute('href', '#i-' + name); svg.append(use); return svg; };
const state = { notes: [], folders: [], selected: null, baseline: '', view: 'all', folder: '', tag: '', mode: 'read', conflict: false, saving: null, ready: false, matches: null, searchID: 0, previewID: 0, openingID: 0, loading: false };
let autosaveTimer, previewTimer, searchTimer, toastTimer;
let session = '';
try {
 const fragment = new URLSearchParams(location.hash.slice(1));
 session = fragment.get('session') || sessionStorage.getItem('neuron-session') || '';
 if (session) sessionStorage.setItem('neuron-session', session);
 if (fragment.has('session')) history.replaceState(null, '', location.pathname);
 const theme = localStorage.getItem('neuron-theme');
 if (theme) document.documentElement.dataset.theme = theme;
} catch (_) { /* Cookies continue to authenticate if storage is unavailable. */ }

async function api(url, options = {}) {
 const response = await fetch(url, { ...options, headers: { 'X-Neuron-Token': session, 'Content-Type': 'application/json', ...options.headers }, credentials: 'same-origin' });
 const result = await response.json();
 if (!response.ok) { const error = new Error(result.error || 'No se pudo completar la operación.'); error.status = response.status; throw error; }
 return result;
}
function notice(message) { $('toast').textContent = message; $('toast').hidden = false; clearTimeout(toastTimer); toastTimer = setTimeout(() => $('toast').hidden = true, 5500); }
function status(message) { $('saveStatus').textContent = message; }
function dirty() { return state.selected && $('editor').value !== state.baseline; }
function dayKey() { const d = new Date(); return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`; }
function dateLabel(value) { return new Intl.DateTimeFormat('es', { day: 'numeric', month: 'short' }).format(new Date(value)); }
function words() { const count = $('editor').value.trim().split(/\s+/).filter(Boolean).length; $('wordCount').textContent = count + (count === 1 ? ' palabra' : ' palabras'); }
function sameFolder(note, folder) { return note.folder === folder || note.folder.startsWith(folder + '/'); }
function filteredNotes() {
 return state.notes.filter(note => {
  if (state.view === 'inbox' && !sameFolder(note, 'Inbox')) return false;
  if (state.view === 'today' && note.title !== 'Daily ' + dayKey()) return false;
  if (state.folder && !sameFolder(note, state.folder)) return false;
  if (state.tag && !(note.tags || []).includes(state.tag)) return false;
  if (state.matches && !state.matches.has(note.path)) return false;
  return true;
 });
}
function renderNavigation() {
 const titles = { all: 'Todas las notas', inbox: 'Inbox', today: 'Hoy', graph: 'Grafo de ideas' };
 $('viewTitle').textContent = state.folder ? state.folder.split('/').pop() : state.tag ? '#' + state.tag : titles[state.view];
 $('viewCaption').textContent = state.view === 'inbox' ? 'Captura ahora. Organiza después.' : state.view === 'today' ? 'Un nuevo día, una página abierta.' : 'Un lugar para cada idea.';
 $('allCount').textContent = state.notes.length;
 $('inboxCount').textContent = state.notes.filter(n => sameFolder(n, 'Inbox')).length;
 document.querySelectorAll('[data-view]').forEach(b => b.classList.toggle('active', b.dataset.view === state.view && !state.folder && !state.tag));
 const folders = $('folders'); folders.replaceChildren();
 if (!state.folders.length) { const text = document.createElement('span'); text.className = 'section-label'; text.textContent = 'Aún sin carpetas'; folders.append(text); }
 for (const folder of state.folders) {
  const button = document.createElement('button'); button.append(icon('folder'));
  const text = document.createElement('span'); text.textContent = folder.split('/').pop(); button.append(text); button.title = folder;
  button.style.paddingLeft = (10 + Math.min(folder.split('/').length-1, 4)*10) + 'px';
  button.classList.toggle('active', state.folder === folder);
  button.onclick = () => setView('all', folder); folders.append(button);
 }
 const tags = [...new Set(state.notes.flatMap(n => n.tags || []))].sort();
 $('tags').replaceChildren();
 for (const tag of tags) { const button = document.createElement('button'); button.className = 'tag'; button.classList.toggle('active', state.tag === tag); button.textContent = '#' + tag; button.title = tag; button.onclick = () => setView('all', '', state.tag === tag ? '' : tag); $('tags').append(button); }
 $('folderOptions').replaceChildren();
 for (const folder of state.folders) { const option = document.createElement('option'); option.value = folder; $('folderOptions').append(option); }
 $('statNotes').textContent = state.notes.length;
 $('statTasks').textContent = state.notes.reduce((sum, n) => sum + n.tasks, 0);
 $('statLinks').textContent = graphEdges(state.notes).length;
}
function renderList() {
 const notes = filteredNotes(); const list = $('noteList'); const scroll = list.scrollTop; list.replaceChildren();
 $('resultCount').textContent = notes.length + (notes.length === 1 ? ' nota' : ' notas') + ' · recientes primero';
 for (const note of notes) {
  const card = document.createElement('button'); card.className = 'note-card'; card.classList.toggle('active', state.selected?.path === note.path); card.setAttribute('aria-pressed', String(state.selected?.path === note.path)); card.title = note.path;
  const folder = document.createElement('div'); folder.className = 'note-folder'; folder.textContent = note.folder || 'Mi bóveda';
  const title = document.createElement('strong'); title.textContent = note.title;
  const excerpt = document.createElement('p'); excerpt.textContent = note.excerpt || 'Una página lista para tu próxima idea.';
  const footer = document.createElement('footer'); const date = document.createElement('span'); date.textContent = dateLabel(note.updated); const tag = document.createElement('span'); tag.textContent = note.tasks ? note.tasks + (note.tasks === 1 ? ' pendiente' : ' pendientes') : (note.tags?.[0] ? '#' + note.tags[0] : 'Markdown'); footer.append(date, tag);
  card.append(folder, title, excerpt, footer); card.onclick = () => openNote(note.path); list.append(card);
 }
 if (!notes.length) {
  const empty = document.createElement('div'); empty.className = 'empty-state'; const heading = document.createElement('strong'); heading.textContent = state.matches ? 'No encontramos esa idea.' : 'Aquí empieza algo nuevo.';
  const text = document.createElement('div'); text.textContent = state.matches ? 'Prueba otras palabras o cambia los filtros.' : 'Crea tu primera nota para este espacio.';
  const button = document.createElement('button'); button.textContent = state.view === 'today' ? 'Crear nota de hoy' : 'Nueva nota'; button.onclick = () => state.view === 'today' ? createDaily() : showCreate(); empty.append(heading,text,button); list.append(empty);
 }
 list.scrollTop = scroll;
}
async function setView(view, folder = '', tag = '') {
 if (!(await ensureSaved())) return;
 state.view = view; state.folder = folder; state.tag = tag;
 const titles = { all: 'Todas las notas', inbox: 'Inbox', today: 'Hoy', graph: 'Grafo de ideas' };
 $('viewTitle').textContent = folder ? folder.split('/').pop() : tag ? '#' + tag : titles[view];
 $('viewCaption').textContent = view === 'inbox' ? 'Captura ahora. Organiza después.' : view === 'today' ? 'Un nuevo día, una página abierta.' : 'Un lugar para cada idea.';
 if (view === 'graph') { $('welcome').hidden = true; $('notePane').hidden = true; $('graphPane').hidden = false; $('breadcrumb').textContent = 'Tu espacio / Grafo de ideas'; renderGraph(); }
 else { $('graphPane').hidden = true; $('welcome').hidden = !!state.selected; $('notePane').hidden = !state.selected; }
 renderNavigation(); renderList();
 if (view === 'today') { const daily = state.notes.find(n => n.title === 'Daily ' + dayKey()); if (daily) await openNote(daily.path); }
}
async function searchNotes() {
 const id = ++state.searchID; const query = $('search').value.trim();
 if (!query) { state.matches = null; renderList(); return; }
 try { const data = await api('/api/notes?q=' + encodeURIComponent(query)); if (id !== state.searchID) return; state.matches = new Set(data.notes.map(n => n.path)); renderList(); }
 catch (error) { notice(error.message); }
}
async function refresh() {
 if (state.loading || document.hidden) return;
 state.loading = true;
 try {
  const data = await api('/api/notes'); state.notes = data.notes; state.folders = data.folders; state.ready = true;
  $('vaultName').textContent = data.vault; $('connection').textContent = 'Tus archivos, en tu equipo';
  if (data.warnings.length && !state.warned) { notice(data.warnings.length + ' archivos no se pudieron leer. Revisa la bóveda con neuron doctor.'); state.warned = true; }
  renderNavigation(); renderList();
  if (state.selected && !state.saving && state.view !== 'graph') {
   const latest = state.notes.find(n => n.path === state.selected.path);
   if (!latest) { setConflict('La nota se movió o eliminó fuera de Neuron. Conservamos tu borrador.'); }
   else if (latest.version !== state.selected.version) {
    if (dirty() || state.conflict) setConflict();
    else { const selectedPath = state.selected.path; const note = await api('/api/note?path=' + encodeURIComponent(selectedPath)); if (state.selected?.path === selectedPath && !dirty() && !state.saving) { displayNote(note, state.mode); status('Actualizada desde disco'); } }
   }
  }
  if (state.view === 'graph') renderGraph();
  if ($('search').value.trim()) await searchNotes();
 } catch (error) {
  $('connection').textContent = 'Sin conexión con Neuron';
  if (!state.ready) { $('resultCount').textContent = 'No se pudo abrir la bóveda'; notice(error.message); }
  if (dirty()) status('Sin conexión · borrador pendiente');
 } finally { state.loading = false; }
}
async function openNote(path) {
 if (!(await ensureSaved())) return;
 const id = ++state.openingID;
 try { const note = await api('/api/note?path=' + encodeURIComponent(path)); if (id !== state.openingID) return; if (dirty() || state.saving || state.conflict) { notice('Guarda tus cambios antes de abrir otra nota.'); return; } if (state.view === 'graph') state.view = 'all'; displayNote(note, 'read'); renderNavigation(); renderList(); }
 catch (error) { notice(error.message); }
}
function displayNote(note, mode) {
 state.previewID++; state.selected = note; state.baseline = note.content || ''; state.conflict = false;
 $('editor').value = state.baseline; $('preview').innerHTML = note.html || '<p class="muted">Esta página espera una idea.</p>';
 $('noteTitle').textContent = note.title; $('noteDate').textContent = 'Actualizada el ' + dateLabel(note.updated);
 $('breadcrumb').textContent = note.path; $('noteTags').replaceChildren();
 for (const tag of note.tags || []) { const span = document.createElement('span'); span.className = 'tag'; span.textContent = '#' + tag; $('noteTags').append(span); }
 $('conflict').hidden = true; $('welcome').hidden = true; $('graphPane').hidden = true; $('notePane').hidden = false; status('Guardado en tu bóveda'); words(); setMode(mode); renderRelated();
}
function setMode(mode) {
 state.mode = mode; $('editorArea').className = 'editor-area mode-' + mode;
 document.querySelectorAll('[data-mode]').forEach(b => { b.classList.toggle('active', b.dataset.mode === mode); b.setAttribute('aria-pressed', String(b.dataset.mode === mode)); });
 if (mode !== 'read') $('editor').focus();
}
function setConflict(message) {
 state.conflict = true; clearTimeout(autosaveTimer); $('conflict').hidden = false;
 $('conflict').querySelector('strong').textContent = message || 'Hay una versión nueva en disco.';
 status('Conflicto · borrador conservado');
}
async function saveNote() {
 if (state.saving) return state.saving;
 if (!dirty()) return !state.conflict;
 if (state.conflict) { notice('Resuelve el conflicto o guarda tu borrador como otra nota.'); return false; }
 const path = state.selected.path, content = $('editor').value, version = state.selected.version;
 status('Guardando…'); $('save').disabled = true;
 state.saving = (async () => {
  try {
   const saved = await api('/api/note', { method: 'PUT', body: JSON.stringify({ path, content, version }) });
   if (state.selected?.path === path) {
    state.selected = saved; state.baseline = content;
    const index = state.notes.findIndex(n => n.path === path); if (index >= 0) state.notes[index] = saved;
    $('noteTitle').textContent = saved.title; $('noteDate').textContent = 'Actualizada el ' + dateLabel(saved.updated);
    status(dirty() ? 'Cambios pendientes…' : 'Guardado en tu bóveda'); renderList(); renderRelated();
   }
   return true;
  } catch (error) {
   if (error.status === 409 || error.status === 404) setConflict();
   else { status('No se pudo guardar · borrador conservado'); notice(error.message); }
   return false;
  } finally { state.saving = null; $('save').disabled = false; }
 })();
 return state.saving;
}
async function ensureSaved() { clearTimeout(autosaveTimer); if (state.saving && !(await state.saving)) return false; if (state.conflict) { notice('Conserva tu borrador o carga la versión en disco antes de cambiar de nota.'); return false; } while (dirty()) { if (!(await saveNote())) return false; } return true; }
async function previewDraft() {
 if (!state.selected) return;
 const id = ++state.previewID;
 try { const data = await api('/api/preview', { method: 'POST', body: JSON.stringify({ path: state.selected.path, content: $('editor').value }) }); if (id === state.previewID) $('preview').innerHTML = data.html || '<p class="muted">Esta página espera una idea.</p>'; }
 catch (error) { if (id === state.previewID) notice(error.message); }
}
function resolveTarget(target, fromPath) {
 let name = target.split('|')[0].split('#')[0].replace(/\.md$/i, '');
 try { name = decodeURIComponent(name); } catch (_) {}
 const fromFolder = fromPath.includes('/') ? fromPath.slice(0, fromPath.lastIndexOf('/')+1) : '';
 const relative = new URL(name + '.md', 'http://neuron/' + fromFolder).pathname.slice(1);
 let exact = state.notes.find(n => n.path === relative || n.path === name + '.md');
 if (exact) return exact;
 const matches = state.notes.filter(n => n.title.toLowerCase() === name.toLowerCase() || n.path.split('/').pop().replace(/\.md$/i,'').toLowerCase() === name.toLowerCase());
 return matches.length === 1 ? matches[0] : null;
}
function graphEdges(notes) {
 const keys = new Set(notes.map(n => n.path)); const edges = []; const seen = new Set();
 for (const n of notes) for (const link of n.links || []) { const target = resolveTarget(link, n.path); if (target && keys.has(target.path) && target.path !== n.path) { const key = [n.path, target.path].sort().join('\0'); if (!seen.has(key)) { seen.add(key); edges.push([n.path, target.path]); } } }
 return edges;
}
function renderRelated() {
 $('relatedNotes').replaceChildren(); if (!state.selected) return;
 const related = new Map();
 for (const link of state.selected.links || []) { const n = resolveTarget(link, state.selected.path); if (n && n.path !== state.selected.path) related.set(n.path, n); }
 for (const n of state.notes) if (n.path !== state.selected.path && (n.links || []).some(link => resolveTarget(link, n.path)?.path === state.selected.path)) related.set(n.path, n);
 for (const note of related.values()) { const button = document.createElement('button'); button.textContent = note.title + ' ↗'; button.onclick = () => openNote(note.path); $('relatedNotes').append(button); }
 if (!related.size) { const p = document.createElement('p'); p.textContent = 'Conecta esta idea con otra usando [[Título de la nota]].'; $('relatedNotes').append(p); }
}
let graphTransform = { x: 0, y: 0, scale: 1 }, drag;
function transformGraph() { $('graphScene').setAttribute('transform', `translate(${graphTransform.x} ${graphTransform.y}) translate(450 325) scale(${graphTransform.scale}) translate(-450 -325)`); }
function zoomGraph(factor) { graphTransform.scale = Math.max(.25, Math.min(4, graphTransform.scale * factor)); transformGraph(); }
function renderGraph() {
 $('graph').classList.toggle('dense', state.notes.length > 35);
 const nodes = state.notes.slice(0,150), edges = graphEdges(nodes), positions = new Map(), scene = $('graphScene'); scene.replaceChildren();
 // Deterministic rings keep the layout stable during file refreshes.
 nodes.forEach((n,i) => { const angle = i * 2.399963229728653; const radius = nodes.length === 1 ? 0 : 245 * Math.sqrt((i+1)/nodes.length); positions.set(n.path, { x: 450 + Math.cos(angle)*radius*1.38, y: 320 + Math.sin(angle)*radius }); });
 function svg(name,attrs) { const e=document.createElementNS('http://www.w3.org/2000/svg',name); for (const [k,v] of Object.entries(attrs)) e.setAttribute(k,String(v)); return e; }
 for (const [a,b] of edges) { const p=positions.get(a),q=positions.get(b); scene.append(svg('line',{x1:p.x,y1:p.y,x2:q.x,y2:q.y,class:'graph-edge'})); }
 for (const n of nodes) {
  const p=positions.get(n.path), degree=edges.filter(e=>e.includes(n.path)).length;
  const g=svg('g',{class:'graph-node',transform:`translate(${p.x} ${p.y})`,tabindex:0,role:'button','aria-label':'Abrir '+n.title});
  const title=svg('title',{});title.textContent=n.title; const circle=svg('circle',{r:Math.min(18,7+degree*2)}); const label=svg('text',{y:Math.min(18,7+degree*2)+19});label.textContent=n.title.length>23?n.title.slice(0,22)+'…':n.title; label.style.fontSize = Math.max(12, Math.min(29, 12*900/Math.max(300,$('graph').clientWidth)))+'px';
  const hit=svg('rect',{x:-90,y:-22,width:180,height:68,fill:'transparent',stroke:'none'}); g.append(title,hit,circle,label); g.onclick=()=>{if(!drag?.moved) openNote(n.path);};g.onkeydown=e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();openNote(n.path);}};scene.append(g);
 }
 $('graphSummary').textContent=nodes.length?`${nodes.length} notas · ${edges.length} conexiones${state.notes.length>150?' · Se muestran las 150 notas más recientes':''} · Enlaces [[wikilink]]`:'Tu mapa empieza con una nota. Conecta dos ideas con [[wikilinks]].';
 transformGraph();
}
async function showCreate() {
 if (!(await ensureSaved())) return;
 $('createForm').reset(); $('createFolder').value = state.folder || (state.view === 'inbox' ? 'Inbox' : ''); $('createError').textContent = '';
 $('createDialog').showModal(); $('createTitle').focus();
}
async function createDaily() {
 if (!(await ensureSaved())) return;
 const exists = state.notes.find(n => n.title === 'Daily ' + dayKey()); if (exists) return openNote(exists.path);
 try { const note = await api('/api/notes', { method:'POST', body:JSON.stringify({title:'Daily '+dayKey(),folder:'',tags:['daily'],content:'## Prioridades de hoy\n- [ ] Definir la siguiente acción\n\n## Notas\n\n## Enlaces\n'}) }); displayNote(note,'edit'); await refresh(); }
 catch(error) { notice(error.message); }
}
$('createForm').onsubmit = async event => {
 event.preventDefault(); const submit = event.submitter; submit.disabled = true;
 try { const note = await api('/api/notes', { method:'POST', body:JSON.stringify({title:$('createTitle').value,folder:$('createFolder').value.trim(),tags:$('createTags').value.split(',').map(t=>t.trim().replace(/^#/, '')).filter(Boolean),content:''}) }); $('createDialog').close(); state.view='all';state.folder='';state.tag='';displayNote(note,'edit');await refresh(); }
 catch(error) { $('createError').textContent=error.message; } finally { submit.disabled=false; }
};
$('editor').addEventListener('input', () => {
 words(); status(state.conflict?'Conflicto · borrador conservado':'Cambios pendientes…');
 clearTimeout(autosaveTimer); clearTimeout(previewTimer);
 previewTimer=setTimeout(previewDraft,220);
 if (!state.conflict) autosaveTimer=setTimeout(async()=>{const ok=await saveNote();if(ok&&dirty()&&!state.conflict)autosaveTimer=setTimeout(saveNote,700);},1000);
});
$('search').oninput=()=>{clearTimeout(searchTimer);state.searchID++;searchTimer=setTimeout(searchNotes,180);};
$('save').onclick=saveNote;
$('newNote').onclick=showCreate;$('welcomeNew').onclick=showCreate;
$('welcomeGraph').onclick=()=>setView('graph');
$('home').onclick=async event=>{event.preventDefault();if(!(await ensureSaved()))return;state.selected=null;state.baseline='';$('editor').value='';state.matches=null;$('search').value='';setView('all');$('breadcrumb').textContent='Tu espacio de ideas';status('Local · Markdown');};
$('theme').onclick=()=>{const theme=document.documentElement.dataset.theme==='dark'?'light':'dark';document.documentElement.dataset.theme=theme;try{localStorage.setItem('neuron-theme',theme);}catch(_){};};
$('helpButton').onclick=()=>$('helpDialog').showModal();
for(const button of document.querySelectorAll('[data-close]'))button.onclick=()=>$(button.dataset.close).close();
for(const button of document.querySelectorAll('[data-view]'))button.onclick=()=>setView(button.dataset.view);
for(const button of document.querySelectorAll('[data-mode]'))button.onclick=()=>setMode(button.dataset.mode);
for(const kind of ['list','card'])$(kind+'View').onclick=()=>{$('noteList').classList.toggle('cards',kind==='card');for(const other of ['list','card']){$(other+'View').classList.toggle('selected',other===kind);$(other+'View').setAttribute('aria-pressed',String(other===kind));}};
$('preview').onclick=event=>{const link=event.target.closest('a[data-wiki]');if(!link)return;event.preventDefault();const target=resolveTarget(link.dataset.wiki,state.selected.path);if(target)openNote(target.path);else notice('No se encontró una nota única para «'+link.dataset.wiki+'». Usa la búsqueda para localizarla.');};
$('copyDraft').onclick=async()=>{
 const source=state.selected;if(!source)return;
 try{const note=await api('/api/notes',{method:'POST',body:JSON.stringify({title:source.title.slice(0,140)+' (borrador)',folder:source.folder,content:$('editor').value,tags:source.tags||[]})});displayNote(note,'edit');await refresh();notice('Borrador conservado en una nueva nota.');}catch(error){notice(error.message);}
};
$('reloadConflict').onclick=async()=>{if(!confirm('¿Cargar la versión en disco y descartar este borrador? Puedes guardarlo como otra nota antes.'))return;try{const note=await api('/api/note?path='+encodeURIComponent(state.selected.path));displayNote(note,state.mode);}catch(error){notice(error.message);}};
$('compareConflict').onclick=async()=>{try{const latest=await api('/api/note?path='+encodeURIComponent(state.selected.path));$('draftVersion').value=$('editor').value;$('diskVersion').value=latest.content||'';$('compareDialog').showModal();}catch(error){notice(error.message);}};
$('zoomIn').onclick=()=>zoomGraph(1.2);$('zoomOut').onclick=()=>zoomGraph(1/1.2);$('resetGraph').onclick=()=>{graphTransform={x:0,y:0,scale:1};transformGraph();};
$('graph').addEventListener('wheel',event=>{event.preventDefault();zoomGraph(event.deltaY<0?1.1:1/1.1);},{passive:false});
$('graph').onpointerdown=event=>{if(event.target.closest('.graph-node'))return;drag={x:event.clientX,y:event.clientY,ox:graphTransform.x,oy:graphTransform.y,moved:false};$('graph').setPointerCapture(event.pointerId);};
$('graph').onpointermove=event=>{if(!drag)return;const rect=$('graph').getBoundingClientRect();const factor=Math.max(900/rect.width,650/rect.height);const dx=event.clientX-drag.x,dy=event.clientY-drag.y;drag.moved=Math.abs(dx)+Math.abs(dy)>4;graphTransform.x=drag.ox+dx*factor;graphTransform.y=drag.oy+dy*factor;transformGraph();};
$('graph').onpointerup=()=>{setTimeout(()=>drag=null,0);};$('graph').onpointercancel=()=>{drag=null;};
window.addEventListener('beforeunload',event=>{if(dirty()||state.saving||state.conflict){event.preventDefault();event.returnValue='';}});
window.addEventListener('keydown',event=>{if(!(event.metaKey||event.ctrlKey))return;const key=event.key.toLowerCase();if(key==='k'){event.preventDefault();$('search').focus();$('search').select();}if(key==='s'){event.preventDefault();saveNote();}if(key==='n'){event.preventDefault();showCreate();}});
refresh();setInterval(refresh,4000);
