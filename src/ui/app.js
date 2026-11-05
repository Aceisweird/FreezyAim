'use strict';

const svg = (size, body) => `<svg viewBox="0 0 24 24" width="${size}" height="${size}" aria-hidden="true">${body}</svg>`;
const ICONS = {
  star: svg(16, '<path d="m12 3.5 2.6 5.4 5.9.8-4.3 4.1 1 5.9-5.2-2.8-5.2 2.8 1-5.9-4.3-4.1 5.9-.8z"/>'),
  copy: svg(16, '<path d="M9 9h11v11H9zM5 15H4V4h11v1"/>'),
  check: svg(14, '<path d="m5 12.5 4.5 4.5L19 7.5"/>'),
  left: svg(20, '<path d="M14.5 6 9 12l5.5 6"/>'),
  right: svg(20, '<path d="M9.5 6 15 12l-5.5 6"/>'),
  target: svg(20, '<circle cx="12" cy="12" r="6.5"/><path d="M12 2.5v5M12 16.5v5M2.5 12h5M16.5 12h5"/>'),
  grid: svg(20, '<rect x="4" y="4" width="7" height="7" rx="2"/><rect x="13" y="4" width="7" height="7" rx="2"/><rect x="4" y="13" width="7" height="7" rx="2"/><rect x="13" y="13" width="7" height="7" rx="2"/>'),
  pen: svg(18, '<path d="m4 20 1.2-4.6L16 4.6a2.1 2.1 0 0 1 3 3L8.2 18.8 4 20zM14.5 6l3.1 3.1"/>'),
  share: svg(16, '<circle cx="17.5" cy="5.5" r="2.5"/><circle cx="6.5" cy="12" r="2.5"/><circle cx="17.5" cy="18.5" r="2.5"/><path d="m8.7 10.7 6.6-3.9M8.7 13.3l6.6 3.9"/>'),
  trash: svg(16, '<path d="M5 7h14M10 7V4.5h4V7M7 7l1 13h8l1-13"/>'),
  sliders: svg(20, '<path d="M4 7.5h9M18 7.5h2M4 16.5h2M11 16.5h9"/><circle cx="15.5" cy="7.5" r="2.5"/><circle cx="8.5" cy="16.5" r="2.5"/>'),
  reticle: '<svg viewBox="0 0 64 64" aria-hidden="true"><circle cx="32" cy="32" r="20.5" stroke-width="4" pathLength="1"/><path d="M32 3v17.5M32 43.5V61M3 32h17.5M43.5 32H61" stroke-width="5"/></svg>',
};

// Every tile uses the same zoom, so a small dot looks smaller than a big cross.
const TILE_ZOOM = 2;

const CODE_PREFIX = 'FA2-';
const OLD_CODE_PREFIX = 'FA1-';
const SHORTCUT_HELP = [
  ['Ctrl+Z, Ctrl+Y', 'Undo and redo'],
  ['/ or Ctrl+F', 'Search'],
  ['P, L, C, B, G, T, D, E', 'Pick a drawing tool on the Draw page'],
  ['S, I, M', 'Select, pick a color, or move the view on the Draw page'],
  ['Delete', 'Remove the selected line on the Draw page'],
  ['[ and ]', 'Change the line width on the Draw page'],
  ['?', 'Show this list'],
];

const page = document.getElementById('page');
const nav = document.getElementById('nav');
const homeButton = document.getElementById('home');
const indicator = document.getElementById('indicator');
const notice = document.getElementById('notice');

let catalog = null; // What never changes, fetched once.
let shown = null; // What the last redraw showed, so only real changes animate.
let navShape = ''; // The sidebar's current list.
let pageKeys = {}; // What this page does with Undo, Redo and single keys.
let flashTimer = null;
const listFilter = { text: '', favorites: false }; // The search on tile pages.
let noticeLeaving = null;
let pillTop = 0;
let activeSection = null; // Which sidebar section the pill is on.
let opening = true; // The opening animation plays once, not on every Home visit.
let previewPool = new Map(); // Reused images, so tiles don't blink when a page redraws.
const iconTemplates = new Map();

// Page building

function el(tag, props = {}, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(props)) {
    if (key.startsWith('on')) node.addEventListener(key.slice(2), value);
    else if (key in node) node[key] = value;
    else node.setAttribute(key, value);
  }
  node.append(...children.flat(2).filter((child) => child !== null && child !== undefined));
  return node;
}

// Each icon is parsed once, then copied, because a page can show hundreds of them.
function icon(markup) {
  if (!iconTemplates.has(markup)) iconTemplates.set(markup, el('template', { innerHTML: markup }));
  return iconTemplates.get(markup).content.firstChild.cloneNode(true);
}

// Crosshairs are drawn at the screen's full resolution, or better, rather than stretched.
function previewSource(config, zoom) {
  const density = Math.max(2, Math.ceil(devicePixelRatio));
  return `/render.png?scale=${zoom * density}&c=${encodeURIComponent(JSON.stringify(config))} ${density}x`;
}

// These stay the same on every page, so they're built once and never blink.
const brandMark = el('img', { src: '/icon.png', alt: '', width: 28, height: 28 });
let logoCache = null;

// The built-in logo is live text, so it reads on any main color.
function logo() {
  const label = `${state.app_name} logo`;
  logoCache ??= state.custom_logo
    ? el('img', { className: 'logo', src: '/logo.png', alt: label })
    : el('div', { className: 'logo-mark', role: 'img', 'aria-label': label }, icon(ICONS.reticle), el('span', {}, state.app_name));
  return logoCache;
}

// Talking to the app

function adopt(next) {
  state = { ...catalog, ...next };
  document.getElementById('theme').textContent = state.theme_css;
  document.documentElement.dataset.motion = state.options.no_animations ? 'off' : 'on';
  document.documentElement.dataset.look = state.options.background ?? '';
  document.querySelector('meta[name="theme-color"]').content = state.title_color;
  syncTitleBar();
}

// The title bar is trimmed from the window on Windows. Its height is measured from
// where the page really starts, so it fits any display scale.
let barHidden = null; // Null until the page has told the app which frame it wants.
let barHeight = 0;

function windowAction(action) {
  request('/api/window', { action, bar: barHeight }).catch(() => {});
}

function syncTitleBar() {
  const hide = state.system_features && !state.options.keep_title_bar;
  document.documentElement.dataset.frame = hide ? 'none' : '';
  if (hide === barHidden) return;
  barHidden = hide;
  barHeight ||= Math.round((outerHeight - innerHeight) * devicePixelRatio);
  // These only run once frames are being drawn, which tells the app the page is on screen.
  requestAnimationFrame(() => requestAnimationFrame(() => windowAction(barHidden ? 'trim' : 'restore')));
}

addEventListener('pointermove', (event) => {
  if (!barHidden) return;
  const measured = Math.round((event.screenY - event.clientY - screenY) * devicePixelRatio);
  if (measured <= 0 || measured === barHeight) return;
  barHeight = measured;
  windowAction('trim');
}, { passive: true });

addEventListener('resize', debounce(() => { if (barHidden) windowAction('trim'); }, 60));

function showNotice(text) {
  if (text) {
    noticeLeaving?.cancel();
    noticeLeaving = null;
    notice.textContent = text;
    notice.hidden = false;
  } else if (calm()) {
    notice.hidden = true;
  } else if (!notice.hidden && !noticeLeaving) {
    noticeLeaving = notice.animate({ opacity: [1, 0], translate: ['0 0', '0 8px'] }, { duration: 170, easing: 'ease-in' });
    noticeLeaving.onfinish = () => {
      notice.hidden = true;
      noticeLeaving = null;
    };
  }
}

// A short message that clears itself.
function flash(text) {
  showNotice(text);
  clearTimeout(flashTimer);
  flashTimer = setTimeout(() => showNotice(state.overlay.problem ?? ''), 2800); // A problem stays up.
}

async function request(path, body) {
  const options = body === undefined ? {} : {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  };
  let response;
  try {
    response = await fetch(path, options);
  } catch {
    throw new Error('Freezy Aim has stopped. Start FreezyAim.exe, then try again.');
  }
  if (response.status === 403) {
    throw new Error('This window has expired. Open it again from the Freezy Aim tray icon.');
  }
  const reply = await response.json();
  if (!response.ok) throw new Error(reply.error);
  return reply;
}

// Runs a change and returns the reply, or nothing if it failed.
async function act(path, body) {
  try {
    const reply = await request(path, body);
    adopt(reply.state);
    showNotice(state.overlay.problem ?? '');
    return reply;
  } catch (error) {
    showNotice(error.message);
    return null;
  }
}

async function actThenRender(path, body) {
  if (await act(path, body)) render('tiles');
}

// Share codes hold a crosshair's name and shape, so people can swap them in a message.

// Short letters replace setting names, anything at its usual value is left out, and each
// point in a drawing is stored as the step from the one before. Then it's all compressed.

const CODE_KEYS = {
  color: 'c', opacity: 'o', length: 'l', gap: 'g', thickness: 't', dot_size: 'd', outline: 'u', outline_color: 'e',
  offset_x: 'x', offset_y: 'y', hide_arms: 'h', rotation: 'r', round_ends: 'n', dot_shape: 's', dot_color: 'a',
  frame: 'f', frame_size: 'z', frame_thickness: 'w', frame_gaps: 'v', frame_color: 'b', glow: 'q', behind: 'i',
  strokes: 'k', merged: 'm', picture: 'p', picture_size: 'P', picture_x: 'X', picture_y: 'Y',
};
const CODE_NAMES = Object.fromEntries(Object.entries(CODE_KEYS).map(([name, letter]) => [letter, name]));
const CODE_USUAL = {
  color: '#FFFFFF', opacity: 1, length: 0, gap: 0, thickness: 0, dot_size: 0, outline: 0, outline_color: '#000000',
  offset_x: 0, offset_y: 0, screen: 0,
};
const CODE_COLORS = new Set(['color', 'outline_color', 'dot_color', 'frame_color']);
const POINT_STEPS = 4; // Points sit on quarter pixels, so they can be whole numbers.

function toBase64(bytes) {
  return btoa(Array.from(bytes, (byte) => String.fromCharCode(byte)).join(''));
}

async function throughStream(bytes, stream) {
  const writer = stream.writable.getWriter();
  writer.write(bytes);
  writer.close();
  return new Uint8Array(await new Response(stream.readable).arrayBuffer());
}

function packStroke({ color, width, points, fill }) {
  let lastX = 0;
  let lastY = 0;
  const steps = points.map((value, index) => {
    const whole = Math.round(value * POINT_STEPS);
    const step = whole - (index % 2 ? lastY : lastX);
    if (index % 2) lastY = whole;
    else lastX = whole;
    return step;
  });
  return [fill ? `*${color.slice(1)}` : color.slice(1), width, ...steps];
}

function unpackStroke([color, width, ...steps]) {
  const totals = [0, 0];
  const points = steps.map((step, index) => {
    totals[index % 2] += step;
    return totals[index % 2] / POINT_STEPS;
  });
  const filled = color.startsWith('*');
  return { color: `#${color.replace('*', '')}`, width, points, ...(filled && { fill: true }) };
}

function packConfig(config) {
  const packed = {};
  for (const [key, value] of Object.entries(config)) {
    if (value === undefined || value === null || value === '' || value === false || value === 0 || key === 'screen') continue;
    if (value === CODE_USUAL[key] || (Array.isArray(value) && !value.length)) continue;
    const letter = CODE_KEYS[key] ?? key;
    if (key === 'strokes') packed[letter] = value.map(packStroke);
    else if (key === 'merged') packed[letter] = value.map(packConfig);
    else packed[letter] = CODE_COLORS.has(key) ? value.slice(1) : value;
  }
  return packed;
}

function unpackConfig(packed) {
  const config = { ...CODE_USUAL };
  for (const [letter, value] of Object.entries(packed)) {
    const key = CODE_NAMES[letter] ?? letter;
    if (key === 'strokes') config[key] = value.map(unpackStroke);
    else if (key === 'merged') config[key] = value.map(unpackConfig);
    else config[key] = CODE_COLORS.has(key) ? `#${value}` : value;
  }
  return config;
}

async function shareCode({ name, config }) {
  const { offset_x: _x, offset_y: _y, ...shape } = config; // Where it sits is your own setting.
  const text = JSON.stringify([name, packConfig(shape)]);
  const packed = await throughStream(new TextEncoder().encode(text), new CompressionStream('deflate-raw'));
  return CODE_PREFIX + toBase64(packed).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '');
}

async function readCode(code) {
  const fromBase64 = (text) => Uint8Array.from(atob(text), (letter) => letter.charCodeAt(0));
  try {
    const text = code.trim();
    if (text.startsWith(OLD_CODE_PREFIX)) {
      return JSON.parse(new TextDecoder().decode(fromBase64(text.slice(OLD_CODE_PREFIX.length))));
    }
    if (!text.startsWith(CODE_PREFIX)) return null;
    const body = text.slice(CODE_PREFIX.length).replaceAll('-', '+').replaceAll('_', '/');
    const unpacked = await throughStream(fromBase64(body), new DecompressionStream('deflate-raw'));
    const [name, packed] = JSON.parse(new TextDecoder().decode(unpacked));
    return { name, config: unpackConfig(packed) };
  } catch {
    return null;
  }
}

async function copyCode(item) {
  try {
    await navigator.clipboard.writeText(await shareCode(item));
    flash('Share code copied.');
  } catch {
    flash("The share code couldn't be copied. Click the window, then try again.");
  }
}

async function importCode() {
  const box = el('input', { className: 'text', type: 'text', 'aria-label': 'Share code', placeholder: `${CODE_PREFIX}...` });
  if (!(await askFirst('Import a crosshair', 'Paste the share code you were sent.', 'Import', { field: box }))) return;
  const shared = await readCode(box.value);
  if (!shared?.config) {
    flash("That isn't a share code. Copy the whole code, then try again.");
    return;
  }
  const reply = await act('/api/crosshair', { name: shared.name, config: shared.config });
  if (reply) {
    render('tiles');
    flash(`Added ${reply.state.crosshairs.find((item) => item.id === reply.id).name}.`);
  }
}

function showShortcuts() {
  const list = el('dl', { className: 'keys' }, SHORTCUT_HELP.flatMap(([keys, action]) => [el('dt', {}, keys), el('dd', {}, action)]));
  askFirst('Keyboard shortcuts', 'These work while this window is open.', 'Done', { field: list, canCancel: false });
}

function find(id) {
  return state.crosshairs.find((item) => item.id === id) ?? state.presets.find((item) => item.id === id);
}

// Tiles and rails

function preview(config) {
  const source = previewSource(config, TILE_ZOOM);
  const spare = previewPool.get(source)?.pop();
  return spare ?? el('img', { srcset: source, alt: '', onload: (event) => event.target.classList.add('ready') });
}

function tile(item, { context, actions = [] }) {
  const inUse = item.id === state.active;
  const favorite = state.favorites.includes(item.id);
  return el('article', { className: inUse ? 'tile in-use' : 'tile', 'data-id': item.id, 'data-context': context },
    el('button', {
      className: 'pick',
      type: 'button',
      'data-key': `${context}-pick-${item.id}`,
      'aria-pressed': String(inUse),
      onclick: () => actThenRender('/api/use', { id: item.id }),
    },
      el('span', { className: 'view' }, preview(item.config)),
      el('span', { className: 'name' }, item.name)),
    inUse ? el('span', { className: shown && shown.active !== item.id ? 'badge fresh' : 'badge', title: 'In use', role: 'img', 'aria-label': 'In use' }, icon(ICONS.check)) : null,
    el('button', {
      className: favorite && shown && !shown.favorites.includes(item.id) ? 'tile-button star fresh' : 'tile-button star',
      type: 'button',
      'aria-label': `${favorite ? 'Remove' : 'Add'} ${item.name} ${favorite ? 'from' : 'to'} favorites`,
      'aria-pressed': String(favorite),
      'data-key': `${context}-star-${item.id}`,
      onclick: () => actThenRender('/api/favorite', { id: item.id, on: !favorite }),
    }, icon(ICONS.star)),
    actions.map(({ kind, label, run }) => el('button', {
      className: `tile-button ${kind}`,
      type: 'button',
      'aria-label': `${label} ${item.name}`,
      onclick: run,
    }, icon(ICONS[kind]))));
}

function chevron(direction, label, track) {
  const sign = direction === 'left' ? -1 : 1;
  return el('button', {
    className: `chevron ${direction}`,
    type: 'button',
    'aria-label': label,
    'data-direction': sign,
    onclick: () => track.scrollBy({ left: sign * track.clientWidth * 0.8 }),
  }, icon(ICONS[direction]));
}

function rail(key, title, ids, hint) {
  const items = ids.map(find).filter(Boolean);
  const heading = `${key}-heading`;
  const track = el('div', { className: 'track', 'data-rail': key, onscroll: (event) => syncRail(event.currentTarget) },
    items.map((item) => tile(item, { context: key })),
    items.length < 3 ? el('p', { className: 'hint' }, hint) : null);
  // The mouse wheel scrolls sideways when there's more to see.
  track.addEventListener('wheel', (event) => {
    if (track.scrollWidth <= track.clientWidth || !event.deltaY) return;
    event.preventDefault();
    glide(track, event.deltaY * 1.6);
  }, { passive: false });
  return el('section', { className: 'row card', 'aria-labelledby': heading },
    el('h2', { id: heading }, title),
    chevron('left', `Scroll ${title.toLowerCase()} left`, track),
    track,
    chevron('right', `Scroll ${title.toLowerCase()} right`, track));
}

// Dims an arrow and softens an edge depending on what's left to scroll to.
function syncRail(track) {
  const room = track.scrollWidth - track.clientWidth;
  const moreLeft = track.scrollLeft > 1;
  const moreRight = track.scrollLeft < room - 1;
  track.classList.toggle('more-left', moreLeft);
  track.classList.toggle('more-right', moreRight);
  for (const button of track.parentElement.querySelectorAll('.chevron')) {
    button.disabled = !(Number(button.dataset.direction) > 0 ? moreRight : moreLeft);
  }
}

function syncRails() {
  for (const track of document.querySelectorAll('.track')) syncRail(track);
}

// Pages

function guide() {
  if (state.options.hide_guide) return null;
  const card = el('section', { className: 'guide card', 'aria-label': 'Getting started' },
    el('ol', {},
      el('li', {}, 'Select any crosshair to put it on your screen.'),
      el('li', {}, 'Run your game in borderless or windowed mode.'),
      el('li', {}, 'Closing this window keeps the crosshair. The red X turns it off.')),
    el('button', {
      className: 'button', type: 'button',
      onclick: async () => { if (await act('/api/options', { hide_guide: true })) card.remove(); },
    }, 'Got it'));
  return card;
}

function homePage() {
  const using = find(state.active);
  return [
    el('header', { className: 'hero card' },
      el('h1', { className: 'welcome', 'aria-label': 'Welcome Back' },
        el('span', {}, 'Welcome'),
        el('span', {}, 'Back')),
      el('div', { className: 'hero-side' },
        logo(),
        using ? el('p', { className: 'using' }, `In use: ${using.name}`) : null,
        el('label', { className: 'quick' }, 'Show crosshair', el('input', {
          className: 'switch', type: 'checkbox', role: 'switch', checked: state.overlay.visible,
          onchange: (event) => act('/api/overlay', { visible: event.target.checked }),
        })))),
    guide(),
    rail('recent', 'Recent crosshairs', state.recent, 'Crosshairs you use will show up here.'),
    rail('favorites', 'Favorites', state.favorites, 'Select the star on a crosshair to keep it here.'),
  ];
}

function matching(items) {
  const wanted = listFilter.text.trim().toLowerCase();
  return items.filter((item) => item.name.toLowerCase().includes(wanted)
    && (!listFilter.favorites || state.favorites.includes(item.id)));
}

// A searchable wall of tiles. A search only redraws the tiles, so you can keep typing.
function tileWall(title, label, items, context, actions, lead, ...tools) {
  const wall = el('section', { className: 'grid card', 'aria-label': label });
  const fill = () => {
    const found = matching(items);
    wall.replaceChildren(...lead, ...found.map((item) => tile(item, { context, actions: actions(item) })));
    if (!found.length && (listFilter.text || listFilter.favorites)) {
      wall.append(el('p', { className: 'hint' }, 'Nothing matches that search.'));
    }
  };
  const refill = () => {
    const before = tilePlaces();
    collectPreviews();
    fill();
    slideTiles(before);
  };
  fill();
  return [
    el('header', { className: 'page-head' },
      el('h1', { className: 'title' }, title),
      el('div', { className: 'find' },
        el('input', {
          className: 'text search', type: 'search', placeholder: 'Search', value: listFilter.text,
          'aria-label': 'Search crosshairs',
          'data-key': 'search',
          oninput: (event) => {
            listFilter.text = event.target.value;
            refill();
          },
        }),
        el('button', {
          className: 'chip', type: 'button', 'aria-pressed': String(listFilter.favorites),
          onclick: (event) => {
            listFilter.favorites = !listFilter.favorites;
            event.currentTarget.setAttribute('aria-pressed', String(listFilter.favorites));
            gainColor(event.currentTarget);
            refill();
          },
        }, 'Favorites'),
        tools)),
    wall,
  ];
}

function minePage() {
  const remove = async (item) => {
    if (await askFirst(`Delete ${item.name}?`, "This can't be undone.", 'Delete')) actThenRender('/api/delete', { id: item.id });
  };
  return tileWall('My crosshairs', 'Saved crosshairs', state.crosshairs, 'mine', (item) => [
    { kind: 'pen', label: 'Edit', run: () => { location.hash = `editor/${item.id}`; } },
    { kind: 'copy', label: 'Duplicate', run: () => actThenRender('/api/copy', { id: item.id }) },
    { kind: 'share', label: 'Copy share code for', run: () => copyCode(item) },
    { kind: 'trash', label: 'Delete', run: () => remove(item) },
  ], [el('button', { className: 'new-tile', type: 'button', onclick: () => { location.hash = 'editor'; } },
    el('b', { 'aria-hidden': 'true' }, '+'), 'New crosshair')],
  el('button', { className: 'button', type: 'button', onclick: importCode }, 'Import code'));
}

function browserPage(category) {
  const items = category
    ? state.presets.filter((item) => item.category === category)
    : state.presets;
  return tileWall(category || 'Crosshair browser', 'Crosshairs', items, 'browser', (item) => [{
    kind: 'pen',
    label: 'Edit a copy of',
    run: async () => {
      const reply = await act('/api/copy', { id: item.id });
      if (reply) location.hash = `editor/${reply.id}`;
    },
  }, { kind: 'share', label: 'Copy share code for', run: () => copyCode(item) }], []);
}

// Sidebar

function route() {
  const [name, ...rest] = decodeURIComponent(location.hash.slice(1)).split('/');
  return { name: name || 'home', detail: rest.join('/') };
}

function navButton(className, target, ...content) {
  return el('button', {
    className,
    type: 'button',
    'data-target': target,
    onclick: () => { location.hash = target; },
  }, ...content);
}

function buildNav() {
  const section = (target, text, glyph) => navButton('nav-item', target, icon(ICONS[glyph]), el('span', {}, text));
  const links = (className, items) => el('ul', { className }, items.map(([target, text]) => el('li', {},
    navButton('', target, el('span', {}, text)))));

  nav.replaceChildren(
    navButton('brand', 'home', brandMark, el('span', {}, state.app_name)),
    section('mine', 'My crosshairs', 'target'),
    links('sub saved', state.crosshairs.slice(0, 3).map((item) => [`editor/${item.id}`, item.name])),
    section('browser', 'Crosshair browser', 'grid'),
    links('sub categories', state.categories.map((category) => [`browser/${encodeURIComponent(category)}`, category])),
    section('editor', 'Crosshair editor', 'pen'),
    links('sub', [['editor/draw', 'Draw']]),
    el('div', { className: 'spacer' }),
    section('settings', 'Settings', 'sliders'));
}

// The sidebar is only rebuilt when its list changes, so its colors can fade as you
// move around.
function renderNav() {
  const shape = JSON.stringify([state.crosshairs.slice(0, 3).map((item) => [item.id, item.name]), state.categories]);
  if (shape !== navShape) {
    navShape = shape;
    buildNav();
  }
  const { name, detail } = route();
  const here = detail ? `${name}/${detail}` : name;
  for (const button of nav.querySelectorAll('button')) {
    const target = decodeURIComponent(button.dataset.target);
    button.toggleAttribute('data-active', button.classList.contains('nav-item') && target === name);
    if (target === here || (target === 'editor/draw' && here.startsWith('editor/draw/'))) button.setAttribute('aria-current', 'page');
    else button.removeAttribute('aria-current');
  }
  moveIndicator();
}

function moveIndicator() {
  const active = nav.querySelector('.nav-item[data-active]');
  indicator.classList.toggle('visible', Boolean(active));
  if (active) {
    indicator.style.transform = `translateY(${active.offsetTop}px)`;
    indicator.style.height = `${active.offsetHeight}px`;
  }
  // The pill fills with color as it glides to a newly selected section.
  const section = active?.textContent ?? null;
  if (active && section !== activeSection && !calm()) {
    indicator.animate(
      [{ backgroundSize: '14% 100%', boxShadow: '0 0 0 transparent' }, { backgroundSize: '100% 100%' }],
      { duration: 560, easing: EASE },
    );
    // The further it travels, the more it stretches on the way.
    const stretch = Math.min(Math.abs(active.offsetTop - pillTop) / 900, 0.24);
    indicator.animate(
      [{ scale: '1 1' }, { scale: `${1 - stretch / 3} ${1 + stretch}`, offset: 0.32 }, { scale: '1 1' }],
      { duration: 560, easing: 'ease-out' },
    );
  }
  pillTop = active?.offsetTop ?? pillTop;
  activeSection = section;
}

// Drawing the current page

function pageContent() {
  const { name, detail } = route();
  if (name === 'mine') return minePage();
  if (name === 'editor') return editorPage(detail);
  if (name === 'browser') return browserPage(detail);
  if (name === 'settings') return settingsPage();
  return homePage();
}

// Keeps the current preview images, so tiles don't blink when they're redrawn.
function collectPreviews() {
  previewPool = new Map();
  for (const image of page.querySelectorAll('.view img')) {
    const source = image.getAttribute('srcset');
    if (!previewPool.has(source)) previewPool.set(source, []);
    previewPool.get(source).push(image);
  }
}

function tilePlaces() {
  return new Map([...page.querySelectorAll('.tile')].map((item) => [
    `${item.dataset.context}:${item.dataset.id}`, item.getBoundingClientRect()]));
}

// Tiles glide from where they were to where they now belong.
function slideTiles(before) {
  if (calm()) return;
  for (const item of page.querySelectorAll('.tile')) {
    const old = before.get(`${item.dataset.context}:${item.dataset.id}`);
    const now = item.getBoundingClientRect();
    if (!old) {
      item.animate({ opacity: [0, 1], scale: [0.88, 1] }, { duration: 480, easing: SPRING });
    } else if (old.left !== now.left || old.top !== now.top) {
      item.animate([{ transform: `translate(${old.left - now.left}px, ${old.top - now.top}px)` }, { transform: 'none' }], { duration: 560, easing: SPRING });
    }
  }
}

function render(motion) {
  const focusKey = document.activeElement?.dataset?.key;
  const scrolled = Object.fromEntries([...document.querySelectorAll('[data-rail]')]
    .map((track) => [track.dataset.rail, track.scrollLeft]));
  const before = motion === 'tiles' ? tilePlaces() : null;

  collectPreviews();
  pageKeys = {};
  renderNav();
  page.replaceChildren(...[pageContent()].flat().filter(Boolean)); // Leaves out optional parts.

  for (const track of document.querySelectorAll('[data-rail]')) {
    track.style.scrollBehavior = 'auto';
    track.scrollLeft = scrolled[track.dataset.rail] ?? 0;
    track.style.scrollBehavior = '';
  }
  syncRails();
  if (focusKey) document.querySelector(`[data-key="${CSS.escape(focusKey)}"]`)?.focus({ preventScroll: true });
  if (motion === 'tiles') {
    slideTiles(before);
    if (shown && shown.active !== state.active) page.querySelectorAll('.tile.in-use .view').forEach(lockOn);
  }
  if (motion === 'page') {
    arrive();
    drawLogo();
  }
  shown = { active: state.active, favorites: [...state.favorites] };
}

async function load() {
  try {
    catalog ??= await request('/api/catalog');
    adopt(await request('/api/state'));
    showNotice(state.overlay.problem ?? '');
  } catch (error) {
    showNotice(error.message);
    return false;
  }
  return true;
}

addEventListener('hashchange', async () => {
  // Opening another crosshair in Draw would replace a new drawing.
  const { name, detail } = route();
  if (name === 'editor' && detail.startsWith('draw/') && !(await settleDrawing('Continue'))) {
    location.hash = 'editor/draw';
    return;
  }
  await saveEditorNow?.();
  listFilter.text = '';
  listFilter.favorites = false;
  if (await load()) {
    changePage(() => {
      page.style.scrollBehavior = 'auto';
      page.scrollTop = 0;
      page.style.scrollBehavior = '';
      render('page');
      page.focus({ preventScroll: true });
    });
  }
});

// Keeps the app window in step with the tray and shortcuts. The editor is left
// alone so a refresh can't interrupt what you're working on.
addEventListener('focus', async () => {
  if (state && route().name !== 'editor' && await load()) render();
});

homeButton.addEventListener('click', () => { location.hash = 'home'; });

for (const action of ['minimize', 'maximize', 'close']) {
  document.getElementById(`window-${action}`).addEventListener('click', () => windowAction(action));
}
const dragStrip = document.getElementById('drag-strip');
dragStrip.addEventListener('pointerdown', (event) => { if (event.button === 0) windowAction('drag'); });
dragStrip.addEventListener('dblclick', () => windowAction('maximize'));

// The red X closes the whole app. The window closes with it, so this message only
// shows if it was opened in a normal browser tab, which a page can't close itself.
document.getElementById('quit').addEventListener('click', async () => {
  if (!(await settleDrawing('Close'))) return;
  await saveEditorNow?.();
  const closing = calm() ? null : document.body.animate(
    { opacity: [1, 0], scale: [1, 0.985], filter: ['blur(0)', 'blur(6px)'] },
    { duration: 200, easing: 'ease-in', fill: 'forwards' },
  );
  await closing?.finished;
  try {
    await request('/api/quit', {});
  } catch {
    // The app may already be gone, which is what was asked for.
  }
  closing?.cancel();
  document.body.replaceChildren(el('p', { className: 'closed' }, 'Freezy Aim has closed. You can close this window.'));
  window.close();
});

// Ctrl+Z and Ctrl+Y undo and redo, and / or Ctrl+F jumps to the search box. Text boxes
// keep their own Undo, and single keys are left alone while you type.
addEventListener('keydown', (event) => {
  if (event.defaultPrevented || document.querySelector('dialog[open]')) return;
  const typing = event.target.matches?.('input[type="text"], input[type="search"], input[type="number"], select');
  const command = event.ctrlKey || event.metaKey;
  const key = event.key.toLowerCase();
  const run = (action) => {
    if (!action) return;
    event.preventDefault();
    action();
  };
  if (command && key === 'z' && !typing) run(event.shiftKey ? pageKeys.redo : pageKeys.undo);
  else if (command && key === 'y' && !typing) run(pageKeys.redo);
  else if ((command && key === 'f') || (key === '/' && !typing)) {
    const search = document.querySelector('.search');
    run(search && (() => { search.focus(); search.select(); }));
  } else if (key === '?' && !typing) {
    showShortcuts();
  } else if (!command && !event.altKey && !typing) pageKeys.letter?.(key);
});

// The browser's right-click menu has nothing useful for the app, except in text boxes.
addEventListener('contextmenu', (event) => {
  if (!event.target.matches('input, textarea')) event.preventDefault();
});

addEventListener('resize', () => {
  if (!state) return;
  syncRails();
  moveIndicator();
});

(async () => {
  if (!(await load())) return;
  render();
  if (opening && route().name === 'home') playOpening();
  else arrive();
  opening = false;
  document.fonts.ready.then(() => { syncRails(); moveIndicator(); });
})();
