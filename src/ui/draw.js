'use strict';

// The Draw page. Lines are kept as shapes rather than pixels, so a drawing stays
// sharp at every zoom and can have an outline and glow like any other crosshair.

const DRAW_SIZE = 432; // Width and height of the drawing area on screen.
const DRAW_TOOLS = [
  ['pen', 'Pen'], ['line', 'Line'], ['circle', 'Circle'], ['box', 'Box'], ['shape', 'Shape'], ['star', 'Star'], ['dot', 'Dot'],
  ['eraser', 'Eraser'], ['select', 'Select'], ['pick', 'Pick color'], ['hand', 'Move'],
];
const DRAW_KEYS = { p: 'pen', l: 'line', c: 'circle', b: 'box', g: 'shape', t: 'star', d: 'dot', e: 'eraser', s: 'select', i: 'pick', m: 'hand' };
const CLOSED_TOOLS = new Set(['circle', 'box', 'shape', 'star']);
const POINTED_TOOLS = new Set(['shape', 'star']);
const LINE_STYLES = [['solid', 'Solid'], ['dashed', 'Dashed'], ['dotted', 'Dotted']];
const STAR_WAIST = 0.45; // How far in a star's inner corners sit.
const PICK_REACH = 2.5; // How close a click has to be to a line, in drawing pixels.
const GROW = 1.25;
const DRAW_ZOOMS = [[3, '3x'], [5, '5x'], [8, '8x'], [12, '12x']];
const MIRRORS = [['off', 'Off'], ['sides', 'Left and right'], ['all', 'All four'], ['eight', 'All eight'], ['spin3', 'Spin 3'], ['spin5', 'Spin 5'], ['spin6', 'Spin 6']];
const MAX_LINES = 300;
const MAX_POINTS = 4000;
const RING_POINTS = 48;
const MAX_PAN = 400; // Stays inside the area a drawing can reach.
const CENTER_PULL = 2; // Shapes started this close to the middle start exactly on it.
const STEADY = 0.45; // How far a smoothed pen moves toward the pointer at each step.
const BLANK_CONFIG = {
  color: '#FFFFFF', opacity: 1, length: 0, gap: 0, thickness: 0, dot_size: 0,
  outline: 0, outline_color: '#000000', offset_x: 0, offset_y: 0, screen: 0,
};

const drawView = { tool: 'pen', width: 2, color: '#00FF00', zoom: 5, mirror: 'off', snap: false, smooth: true, backdrop: 'dark', panX: 0, panY: 0, sides: 6, fill: false, style: 'solid' };

let drawing = null; // Kept when you leave the page, so nothing is lost.

const tidy = (value) => Math.round(value * 4) / 4;

function startDrawing(item) {
  const from = item ?? state.draft;
  const config = from ? structuredClone(from.config) : { ...BLANK_CONFIG };
  config.strokes ??= [];
  return { id: item?.id ?? '', name: from?.name ?? 'My drawing', config, undo: [], redo: [], dirty: !item && Boolean(state.draft) };
}

async function discardDraft() {
  state.draft = null;
  await request('/api/draft', {}).catch(() => {});
}

// A new drawing is kept on disk as you go, so it comes back if the app closes.
const keepDraft = debounce(() => {
  if (!drawing || drawing.id) return;
  if (hasUnsavedWork()) request('/api/draft', { name: drawing.name, config: drawing.config }).catch(() => {});
  else discardDraft();
}, 400);

function hasShape(config) {
  return (config.length > 0 && config.thickness > 0) || config.dot_size > 0 || Boolean(config.frame) || Boolean(config.merged?.length) || Boolean(config.picture);
}

function hasUnsavedWork() {
  return Boolean(drawing?.dirty) && (drawing.config.strokes.length > 0 || hasShape(drawing.config));
}

// Offers to save a new drawing before it could be lost. Returns false to stay put.
async function settleDrawing(verb) {
  if (drawing?.id || !hasUnsavedWork()) return true;
  const answer = await askFirst('Save your drawing?', "It hasn't been saved, so you'd lose it.", 'Save', { also: `${verb} without saving` });
  if (answer === 'also') {
    drawing = null;
    await discardDraft();
    return true;
  }
  return answer === true && Boolean(await drawing.save());
}

// Closing the window or reloading can't show the app's own question, so the browser asks.
addEventListener('beforeunload', (event) => {
  if (hasUnsavedWork()) event.preventDefault();
});

function gapToLine([x, y], [fromX, fromY], [toX, toY]) {
  const alongX = toX - fromX;
  const alongY = toY - fromY;
  const length = alongX * alongX + alongY * alongY;
  const share = length ? Math.min(Math.max(((x - fromX) * alongX + (y - fromY) * alongY) / length, 0), 1) : 0;
  return Math.hypot(x - fromX - share * alongX, y - fromY - share * alongY);
}

// Drops points a line doesn't need, which keeps drawings small and fast.
function simplify(points, tolerance = 0.2) {
  if (points.length < 3) return points;
  const keep = new Set([0, points.length - 1]);
  const spans = [[0, points.length - 1]];
  while (spans.length) {
    const [first, last] = spans.pop();
    let furthest = -1;
    let worst = tolerance;
    for (let index = first + 1; index < last; index += 1) {
      const gap = gapToLine(points[index], points[first], points[last]);
      if (gap > worst) {
        worst = gap;
        furthest = index;
      }
    }
    if (furthest >= 0) {
      keep.add(furthest);
      spans.push([first, furthest], [furthest, last]);
    }
  }
  return points.filter((_, index) => keep.has(index));
}

function pairs(flat) {
  return Array.from({ length: flat.length / 2 }, (_, index) => [flat[index * 2], flat[index * 2 + 1]]);
}

function ring([centerX, centerY], radius) {
  return Array.from({ length: RING_POINTS + 1 }, (_, index) => {
    const angle = (index / RING_POINTS) * 2 * Math.PI;
    return [tidy(centerX + radius * Math.cos(angle)), tidy(centerY + radius * Math.sin(angle))];
  });
}

// Holding Shift keeps a line to eight directions.
function lineEnd(start, end, locked) {
  if (!locked) return end;
  const step = Math.PI / 4;
  const angle = Math.round(Math.atan2(end[1] - start[1], end[0] - start[0]) / step) * step;
  const length = Math.hypot(end[0] - start[0], end[1] - start[1]);
  return [tidy(start[0] + length * Math.cos(angle)), tidy(start[1] + length * Math.sin(angle))];
}

function box([fromX, fromY], [toX, toY]) {
  return [[fromX, fromY], [toX, fromY], [toX, toY], [fromX, toY], [fromX, fromY]];
}

// A regular shape or star, from its middle out to one corner.
function pointed(center, corner, sides, star) {
  const radius = gapToLine(corner, center, center);
  const start = Math.atan2(corner[1] - center[1], corner[0] - center[0]);
  const count = star ? sides * 2 : sides;
  return Array.from({ length: count + 1 }, (_, index) => {
    const reach = star && index % 2 ? radius * STAR_WAIST : radius;
    const angle = start + (index / count) * 2 * Math.PI;
    return [tidy(center[0] + reach * Math.cos(angle)), tidy(center[1] + reach * Math.sin(angle))];
  });
}

// Cuts a line into dashes or dots.
function dashes(points, style, width) {
  const on = style === 'dashed' ? width * 3 : 0;
  const off = width * 2 + (style === 'dashed' ? 0 : 1);
  const spotAt = (from, to, share) => [tidy(from[0] + (to[0] - from[0]) * share), tidy(from[1] + (to[1] - from[1]) * share)];
  const pieces = [];
  let piece = [points[0]];
  let drawn = true;
  let left = on;
  for (let index = 1; index < points.length; index += 1) {
    const from = points[index - 1];
    const to = points[index];
    const length = gapToLine(from, to, to);
    let walked = 0;
    while (length - walked >= left) {
      walked += left;
      const spot = spotAt(from, to, length ? walked / length : 0);
      if (drawn) {
        piece.push(spot);
        pieces.push(piece);
      } else {
        piece = [spot];
      }
      drawn = !drawn;
      left = drawn ? on : off;
      if (!left && drawn) { // A dot has no length, so it's finished as soon as it starts.
        pieces.push(piece);
        drawn = false;
        left = off;
      }
    }
    left -= length - walked;
    if (drawn) piece.push(to);
  }
  if (drawn && piece.length > 1) pieces.push(piece);
  return pieces;
}

function strokesFrom(points) {
  const { color, width, style, tool } = drawView;
  const fill = drawView.fill && CLOSED_TOOLS.has(tool);
  return mirrored(points).flatMap((each) => {
    if (fill) return [{ color, width, points: each.flat(), fill: true }];
    if (style === 'solid' || each.length < 2) return [{ color, width, points: each.flat() }];
    return dashes(each, style, width).map((piece) => ({ color, width, points: piece.flat() }));
  });
}

function insideShape([x, y], points) {
  let inside = false;
  for (let index = 0; index < points.length; index += 1) {
    const [fromX, fromY] = points[index];
    const [toX, toY] = points[(index + 1) % points.length];
    if ((fromY > y) !== (toY > y) && x < fromX + ((y - fromY) / (toY - fromY)) * (toX - fromX)) inside = !inside;
  }
  return inside;
}

// Finds the line under a point, starting from the one on top.
function strokeAt(strokes, point) {
  for (let index = strokes.length - 1; index >= 0; index -= 1) {
    const stroke = strokes[index];
    const spots = pairs(stroke.points);
    const reach = stroke.width / 2 + PICK_REACH;
    if (spots.length === 1 && gapToLine(point, spots[0], spots[0]) < reach) return index;
    if (spots.slice(1).some((spot, at) => gapToLine(point, spots[at], spot) < reach)) return index;
    if (stroke.fill && insideShape(point, spots)) return index;
  }
  return -1;
}

function boundsOf(strokes) {
  const spots = strokes.flatMap((stroke) => pairs(stroke.points));
  const edge = (axis, pick) => pick(...spots.map((each) => each[axis]));
  return { left: edge(0, Math.min), right: edge(0, Math.max), top: edge(1, Math.min), bottom: edge(1, Math.max) };
}

function mirrored(points) {
  const flip = (signX, signY) => points.map(([x, y]) => [x * signX + 0, y * signY + 0]);
  if (drawView.mirror === 'off') return [points];
  if (drawView.mirror.startsWith('spin')) {
    const count = Number(drawView.mirror.slice(4));
    return Array.from({ length: count }, (_, turn) => {
      const angle = (turn / count) * 2 * Math.PI;
      const cos = Math.cos(angle);
      const sin = Math.sin(angle);
      return points.map(([x, y]) => [tidy(x * cos - y * sin), tidy(x * sin + y * cos)]);
    });
  }
  if (drawView.mirror === 'sides') return [points, flip(-1, 1)];
  const four = [points, flip(-1, 1), flip(1, -1), flip(-1, -1)];
  if (drawView.mirror === 'all') return four;
  return [...four, ...four.map((each) => each.map(([x, y]) => [y, x]))];
}

// Removes the part of a line inside a circle, which can split it in two.
function cut(stroke, center, radius) {
  const reach = radius + stroke.width / 2;
  const points = pairs(stroke.points);
  if (points.length === 1) return gapToLine(points[0], center, center) < reach ? [] : [stroke];
  const touched = points.slice(1).some((point, index) => gapToLine(center, points[index], point) < reach);
  if (!touched) return [stroke];
  const pieces = [[]];
  points.slice(1).forEach(([toX, toY], index) => {
    const [fromX, fromY] = points[index];
    const steps = Math.max(Math.ceil(Math.hypot(toX - fromX, toY - fromY) * 2), 1);
    for (let step = 0; step <= steps; step += 1) {
      const spot = [tidy(fromX + ((toX - fromX) * step) / steps), tidy(fromY + ((toY - fromY) * step) / steps)];
      if (gapToLine(spot, center, center) >= reach) pieces.at(-1).push(spot);
      else if (pieces.at(-1).length) pieces.push([]);
    }
  });
  return pieces.filter((piece) => piece.length > 1).map((piece) => ({ ...stroke, points: simplify(piece).flat() }));
}

function drawPage(id) {
  const item = state.crosshairs.find((each) => each.id === id);
  if (drawing?.id !== (item?.id ?? '')) {
    drawing = startDrawing(item);
    if (drawing.dirty) flash('Your unsaved drawing is back.');
  } else if (item && !drawing.dirty) {
    // Picks up anything changed in the editor since this drawing was last open.
    drawing = { ...startDrawing(item), undo: drawing.undo, redo: drawing.redo };
  }
  const work = drawing;
  let lastTool = drawView.tool === 'pick' ? 'pen' : drawView.tool;
  const ratio = Math.max(devicePixelRatio, 1);
  const canvas = el('canvas', {
    width: DRAW_SIZE * ratio,
    height: DRAW_SIZE * ratio,
    role: 'img',
    'aria-label': 'Drawing area',
    onpointerdown: begin,
    onpointermove: extend,
    onpointerup: finish,
    onmousedown: (event) => { if (event.button === 1) event.preventDefault(); }, // Stops the browser's autoscroll.
    onpointercancel: () => {
      panning = null;
      if (line || lift) work.config.strokes = JSON.parse(before); // Puts back anything half erased.
      line = null;
      lift = null;
      paint();
    },
    onpointerleave: () => { hover = null; paint(); },
    onwheel: (event) => {
      event.preventDefault();
      const zooms = DRAW_ZOOMS.map(([zoom]) => zoom);
      const next = zooms[zooms.indexOf(drawView.zoom) + (event.deltaY < 0 ? 1 : -1)];
      if (next) document.querySelector(`[data-choice="zoom-${next}"]`)?.click();
    },
  });
  const under = el('img', { alt: '' });
  let underShows = '';
  const actual = el('img', { alt: 'Actual size preview of your drawing' });
  const stage = el('figure', {}, under, canvas);
  const actualStage = el('figure', {}, actual);
  const undoButton = el('button', { className: 'button', type: 'button', onclick: () => stepBack(work.undo, work.redo) }, 'Undo');
  const redoButton = el('button', { className: 'button', type: 'button', onclick: () => stepBack(work.redo, work.undo) }, 'Redo');
  const saveButton = el('button', { className: 'button primary', type: 'button', onclick: save });
  const useButton = el('button', { type: 'button', onclick: useDrawing });
  const editButton = el('button', {
    className: 'button', type: 'button',
    onclick: async () => { if (!work.dirty || await save()) location.hash = `editor/${work.id}`; },
  }, 'Edit settings');
  let line = null; // The line being drawn right now.
  let panning = null; // Where a drag that moves the view began.
  let selected = -1; // The line the Select tool is holding.
  let lift = null; // Where a drag that moves that line began.
  let lastTouch = 0;
  let before = ''; // The drawing as it was when that line started.
  let hover = null;
  const saveNote = el('span', { className: 'save-state', role: 'status' });
  const resetView = el('button', { className: 'button', type: 'button', onclick: () => setPan(0, 0) }, 'Reset view');

  work.save = save;

  function paint() {
    const context = canvas.getContext('2d');
    const size = canvas.width;
    const scale = ratio * drawView.zoom;
    context.setTransform(1, 0, 0, 1, 0, 0);
    context.clearRect(0, 0, size, size);
    context.setTransform(scale, 0, 0, scale, size / 2 + drawView.panX * scale, size / 2 + drawView.panY * scale);
    const half = size / 2 / scale;
    const left = -half - drawView.panX;
    const right = half - drawView.panX;
    const top = -half - drawView.panY;
    const bottom = half - drawView.panY;

    context.lineCap = 'round';
    context.lineJoin = 'round';
    context.strokeStyle = 'rgb(128 128 128 / 0.16)';
    context.lineWidth = 1 / scale;
    context.beginPath();
    if (drawView.zoom >= 8) {
      for (let spot = Math.ceil(left); spot < right; spot += 1) {
        context.moveTo(spot, top);
        context.lineTo(spot, bottom);
      }
      for (let spot = Math.ceil(top); spot < bottom; spot += 1) {
        context.moveTo(left, spot);
        context.lineTo(right, spot);
      }
    }
    context.stroke();
    context.strokeStyle = 'rgb(128 128 128 / 0.55)';
    context.beginPath();
    context.moveTo(0, top);
    context.lineTo(0, bottom);
    context.moveTo(left, 0);
    context.lineTo(right, 0);
    context.stroke();
    context.rotate(turned()); // Rotation in the editor turns drawn lines too.

    const trace = (flat, width, color, filled) => {
      context.strokeStyle = color;
      context.fillStyle = color;
      context.lineWidth = width;
      context.beginPath();
      context.moveTo(flat[0], flat[1]);
      for (let index = 2; index < flat.length; index += 2) context.lineTo(flat[index], flat[index + 1]);
      if (flat.length === 2) context.lineTo(flat[0], flat[1]);
      if (filled) context.fill();
      context.stroke();
    };
    const lines = [...work.config.strokes];
    if (line && drawView.tool !== 'eraser') lines.push(...strokesFrom(line.points));
    const { outline = 0, glow = 0, outline_color: edge } = work.config;
    if (glow) {
      context.shadowBlur = glow * scale;
      for (const each of lines) {
        context.shadowColor = each.color;
        trace(each.points, each.width, each.color, each.fill);
      }
      context.shadowBlur = 0;
    }
    if (outline) for (const each of lines) trace(each.points, each.width + outline * 2, edge, each.fill);
    for (const each of lines) trace(each.points, each.width, each.color, each.fill);
    if (work.config.strokes[selected]) {
      const stroke = work.config.strokes[selected];
      const edges = boundsOf([stroke]);
      const pad = stroke.width / 2 + 1;
      context.strokeStyle = '#19d3e6';
      context.lineWidth = 1 / drawView.zoom;
      context.setLineDash([4 / drawView.zoom, 3 / drawView.zoom]);
      context.strokeRect(edges.left - pad, edges.top - pad, edges.right - edges.left + pad * 2, edges.bottom - edges.top + pad * 2);
      context.setLineDash([]);
    }
    if (hover && !['hand', 'select', 'pick'].includes(drawView.tool)) {
      context.strokeStyle = 'rgb(128 128 128 / 0.9)';
      context.lineWidth = 1 / scale;
      context.beginPath();
      context.arc(hover[0], hover[1], Math.max(drawView.width / 2, drawView.tool === 'eraser' ? 1.5 : 0.5), 0, 2 * Math.PI);
      context.stroke();
    }
  }

  function turned() { return ((work.config.rotation ?? 0) * Math.PI) / 180; }

  function spot(event) {
    const box = canvas.getBoundingClientRect();
    const scale = box.width / DRAW_SIZE;
    const place = (offset, pan) => (offset / scale - DRAW_SIZE / 2) / drawView.zoom - pan;
    const snapped = (value) => {
      if (!drawView.snap) return tidy(value);
      const half = drawView.width % 2 ? 0.5 : 0;
      return Math.round(value - half) + half;
    };
    const screenX = place(event.clientX - box.left, drawView.panX);
    const screenY = place(event.clientY - box.top, drawView.panY);
    const cos = Math.cos(turned());
    const sin = Math.sin(turned());
    return [snapped(screenX * cos + screenY * sin), snapped(screenY * cos - screenX * sin)];
  }

  function erase([x, y]) {
    const radius = Math.max(drawView.width / 2, 1.5);
    for (const [center] of mirrored([[x, y]])) {
      work.config.strokes = work.config.strokes.flatMap((stroke) => cut(stroke, center, radius));
    }
  }

  function setPan(x, y) {
    drawView.panX = Math.min(Math.max(x, -MAX_PAN), MAX_PAN);
    drawView.panY = Math.min(Math.max(y, -MAX_PAN), MAX_PAN);
    under.style.transform = `translate(${drawView.panX * drawView.zoom}px, ${drawView.panY * drawView.zoom}px)`;
    resetView.disabled = !drawView.panX && !drawView.panY;
    paint();
  }

  function begin(event) {
    if (event.button === 1 || (event.button === 0 && drawView.tool === 'hand')) {
      canvas.setPointerCapture(event.pointerId);
      panning = { x: event.clientX, y: event.clientY, panX: drawView.panX, panY: drawView.panY };
      stage.classList.add('panning');
      return;
    }
    if (event.button !== 0) return;
    canvas.setPointerCapture(event.pointerId);
    const point = spot(event);
    if (drawView.tool === 'pick') {
      const found = work.config.strokes[strokeAt(work.config.strokes, point)];
      if (found) {
        useColor(found.color);
        document.querySelector(`[data-choice="tool-${lastTool}"]`)?.click();
      }
      return;
    }
    if (drawView.tool === 'select') {
      selected = strokeAt(work.config.strokes, point);
      before = JSON.stringify(work.config.strokes);
      lift = selected >= 0 ? { from: point, points: [...work.config.strokes[selected].points] } : null;
      refresh();
      return;
    }
    const pulled = Math.hypot(...point) < CENTER_PULL && drawView.tool !== 'pen' ? [0, 0] : point;
    before = JSON.stringify(work.config.strokes);
    line = { start: pulled, points: [pulled] };
    if (drawView.tool === 'eraser') erase(point);
    paint();
  }

  function extend(event) {
    if (panning) {
      const scale = (canvas.getBoundingClientRect().width / DRAW_SIZE) * drawView.zoom;
      setPan(panning.panX + (event.clientX - panning.x) / scale, panning.panY + (event.clientY - panning.y) / scale);
      return;
    }
    hover = spot(event);
    if (lift) {
      const stroke = work.config.strokes[selected];
      const moveX = hover[0] - lift.from[0];
      const moveY = hover[1] - lift.from[1];
      stroke.points = lift.points.map((value, index) => tidy(value + (index % 2 ? moveY : moveX)));
    }
    if (line) {
      const moves = event.getCoalescedEvents?.() ?? [];
      for (const move of moves.length ? moves : [event]) {
        const point = spot(move);
        if (drawView.tool === 'pen') {
          const last = line.points.at(-1);
          const steady = drawView.smooth && !drawView.snap;
          const next = steady ? last.map((value, index) => tidy(value + (point[index] - value) * STEADY)) : point;
          if (gapToLine(next, last, last) >= 0.25) line.points.push(next);
          line.end = point;
        } else if (drawView.tool === 'line') {
          line.points = [line.start, lineEnd(line.start, point, event.shiftKey)];
        } else if (drawView.tool === 'circle') {
          line.points = ring(line.start, gapToLine(point, line.start, line.start));
        } else if (drawView.tool === 'box') {
          line.points = box(line.start, point);
        } else if (POINTED_TOOLS.has(drawView.tool)) {
          line.points = pointed(line.start, point, drawView.sides, drawView.tool === 'star');
        } else if (drawView.tool === 'dot') {
          line.points = [point];
        } else {
          erase(point);
        }
      }
    }
    paint();
  }

  function finish() {
    panning = null;
    stage.classList.remove('panning');
    if (lift) {
      lift = null;
      if (JSON.stringify(work.config.strokes) !== before) {
        work.undo.push(before);
        work.redo.length = 0;
        changed();
      }
      return;
    }
    if (!line) return;
    const { points, end } = line;
    line = null;
    if (drawView.tool !== 'eraser') {
      if (end && gapToLine(end, points.at(-1), points.at(-1)) > 0) points.push(end); // A smoothed line still ends at the pointer.
      const added = strokesFrom(simplify(points));
      const used = work.config.strokes.reduce((total, stroke) => total + stroke.points.length / 2, 0);
      const adding = added.reduce((total, each) => total + each.points.length / 2, 0);
      if (work.config.strokes.length + added.length > MAX_LINES || used + adding > MAX_POINTS) {
        flash('This drawing is full. Erase some lines to keep drawing.');
      } else {
        work.config.strokes.push(...added);
      }
    }
    if (JSON.stringify(work.config.strokes) === before) {
      paint();
      return;
    }
    work.undo.push(before);
    work.redo.length = 0;
    changed();
  }

  function stepBack(from, to) {
    if (!from.length || line) return;
    to.push(JSON.stringify(work.config.strokes));
    work.config.strokes = JSON.parse(from.pop());
    selected = -1;
    changed();
  }

  // Changing the held line with a slider or swatch counts as one step for Undo.
  function changeSelected(change) {
    const stroke = work.config.strokes[selected];
    if (!stroke) return false;
    const now = Date.now();
    if (now - lastTouch > 700) {
      work.undo.push(JSON.stringify(work.config.strokes));
      work.redo.length = 0;
    }
    lastTouch = now;
    change(stroke);
    changed();
    return true;
  }

  // Flips, turns and resizes work on the held line, or on the whole drawing.
  function transform(move, resize = 1) {
    const chosen = work.config.strokes[selected] ? [work.config.strokes[selected]] : work.config.strokes;
    if (!chosen.length) return;
    const edges = boundsOf(chosen);
    const middleX = selected >= 0 ? (edges.left + edges.right) / 2 : 0;
    const middleY = selected >= 0 ? (edges.top + edges.bottom) / 2 : 0;
    const moved = chosen.map((stroke) => pairs(stroke.points).flatMap(([x, y]) => {
      const [nextX, nextY] = move(x - middleX, y - middleY);
      return [tidy(nextX + middleX), tidy(nextY + middleY)];
    }));
    if (moved.flat().some((value) => Math.abs(value) > MAX_PAN + 100)) {
      flash("That would go too far from the center.");
      return;
    }
    work.undo.push(JSON.stringify(work.config.strokes));
    work.redo.length = 0;
    chosen.forEach((stroke, index) => {
      stroke.points = moved[index];
      stroke.width = Math.min(Math.max(tidy(stroke.width * resize), 1), 40);
    });
    changed();
  }

  function turnBy(degrees) {
    const angle = (degrees * Math.PI) / 180;
    const cos = Math.cos(angle);
    const sin = Math.sin(angle);
    transform((x, y) => [x * cos - y * sin, x * sin + y * cos]);
  }

  function reorder(change) {
    const { strokes } = work.config;
    if (!strokes[selected]) return;
    work.undo.push(JSON.stringify(strokes));
    work.redo.length = 0;
    const [stroke] = strokes.splice(selected, 1);
    selected = change(strokes, stroke);
    strokes.splice(selected, 0, stroke);
    changed();
  }

  function duplicateSelected() {
    const stroke = work.config.strokes[selected];
    if (!stroke) return;
    if (work.config.strokes.length >= MAX_LINES) {
      flash('This drawing is full. Erase some lines to keep drawing.');
      return;
    }
    work.undo.push(JSON.stringify(work.config.strokes));
    work.redo.length = 0;
    work.config.strokes.push({ ...stroke, points: stroke.points.map((value) => value + 4) });
    selected = work.config.strokes.length - 1;
    changed();
  }

  function deleteSelected() {
    if (!work.config.strokes[selected]) return;
    work.undo.push(JSON.stringify(work.config.strokes));
    work.redo.length = 0;
    work.config.strokes.splice(selected, 1);
    selected = -1;
    changed();
  }

  // Moves the whole drawing so it sits exactly on the aiming point.
  function center() {
    const spots = work.config.strokes.flatMap((stroke) => pairs(stroke.points));
    if (!spots.length) return;
    const middle = (axis) => {
      const values = spots.map((each) => each[axis]);
      return tidy((Math.min(...values) + Math.max(...values)) / 2);
    };
    const moveX = middle(0);
    const moveY = middle(1);
    if (!moveX && !moveY) return;
    work.undo.push(JSON.stringify(work.config.strokes));
    work.redo.length = 0;
    work.config.strokes = work.config.strokes.map((stroke) => ({
      ...stroke,
      points: stroke.points.map((value, index) => value - (index % 2 ? moveY : moveX)),
    }));
    changed();
  }

  function clear() {
    if (!work.config.strokes.length) return;
    work.undo.push(JSON.stringify(work.config.strokes));
    work.redo.length = 0;
    work.config.strokes = [];
    selected = -1;
    changed();
  }

  function changed() {
    work.dirty = true;
    saveEditorNow = work.id ? save : null; // Saved crosshairs save as you leave.
    if (!work.id) keepDraft();
    refresh();
  }

  async function save() {
    saveEditorNow = null;
    if (!work.config.strokes.length && !hasShape(work.config)) {
      flash('Draw something first.');
      return null;
    }
    const reply = await act('/api/crosshair', { id: work.id, name: work.name, config: work.config });
    if (!reply) return null;
    if (!work.id) {
      work.id = reply.id;
      if (route().detail === 'draw') history.replaceState(null, '', `#editor/draw/${reply.id}`);
      await discardDraft();
    }
    work.dirty = false;
    renderNav();
    refresh();
    return reply;
  }

  async function useDrawing() {
    if ((work.dirty || !work.id) && !(await save())) return;
    if (!(await act('/api/use', { id: work.id }))) return;
    refresh();
    gainColor(useButton);
    lockOn(stage);
  }

  function refresh() {
    under.hidden = !hasShape(work.config);
    if (!under.hidden) showUnder();
    actual.srcset = previewSource(work.config, 1);
    stage.className = `stage draw-stage ${drawView.backdrop} ${drawView.tool === 'hand' ? 'hand' : ''}`;
    actualStage.className = `stage actual ${drawView.backdrop}`;
    undoButton.disabled = work.undo.length === 0;
    redoButton.disabled = work.redo.length === 0;
    const saved = Boolean(work.id) && !work.dirty;
    saveButton.disabled = saved;
    saveButton.textContent = saved ? 'Saved' : 'Save crosshair';
    saveNote.textContent = hasUnsavedWork() ? (work.id ? 'Saves when you leave' : 'Not saved yet') : '';
    editButton.disabled = !work.id;
    const inUse = Boolean(work.id) && state.active === work.id;
    useButton.className = `button ${inUse ? 'good' : ''}`;
    useButton.textContent = inUse ? 'In use' : 'Use crosshair';
    if (drawView.tool !== 'select') selected = -1;
    sidesRow.hidden = !POINTED_TOOLS.has(drawView.tool);
    fillRow.hidden = !CLOSED_TOOLS.has(drawView.tool);
    selectedTools.hidden = !work.config.strokes[selected];
    const placing = !['hand', 'select', 'pick', 'eraser'].includes(drawView.tool);
    stage.classList.toggle('picking', !placing && drawView.tool !== 'hand');
    if (drawView.tool !== 'pick') lastTool = drawView.tool;
    setPan(drawView.panX, drawView.panY);
  }

  // Large previews are capped, so the real size is asked for to keep them lined up.
  async function showUnder() {
    const shape = { ...work.config, strokes: undefined };
    const shows = JSON.stringify(shape) + drawView.zoom;
    if (shows === underShows) return;
    underShows = shows;
    const code = encodeURIComponent(JSON.stringify(shape));
    const density = Math.max(2, Math.ceil(devicePixelRatio));
    const size = await request(`/api/measure?scale=${drawView.zoom * density}&c=${code}`).catch(() => null);
    if (!size || shows !== underShows) return;
    under.srcset = previewSource(shape, drawView.zoom);
    under.style.width = `${(size.side * drawView.zoom) / size.scale}px`;
    under.style.height = under.style.width;
  }

  const choice = (label, options, key) => {
    const buttons = options.map(([value, text]) => el('button', {
      className: 'chip',
      type: 'button',
      'aria-pressed': String(drawView[key] === value),
      'data-choice': `${key}-${value}`,
      onclick: (event) => {
        drawView[key] = value;
        for (const button of buttons) button.setAttribute('aria-pressed', String(button === event.currentTarget));
        gainColor(event.currentTarget);
        refresh();
      },
    }, text));
    return el('div', { className: 'control' },
      el('span', { className: 'control-label' }, label),
      el('div', { className: 'chips', role: 'group', 'aria-label': label }, buttons));
  };

  const widthOutput = el('output', {}, String(drawView.width));
  const widthInput = el('input', {
    type: 'range', min: 1, max: 16, value: drawView.width,
    oninput: (event) => {
      drawView.width = Number(event.target.value);
      widthOutput.textContent = event.target.value;
      setFill(event.target);
      changeSelected((stroke) => { stroke.width = drawView.width; });
    },
  });
  setFill(widthInput);

  const sidesOutput = el('output', {}, String(drawView.sides));
  const sidesInput = el('input', {
    type: 'range', min: 3, max: 12, value: drawView.sides,
    oninput: (event) => {
      drawView.sides = Number(event.target.value);
      sidesOutput.textContent = event.target.value;
      setFill(event.target);
    },
  });
  setFill(sidesInput);
  const sidesRow = el('label', { className: 'field' }, el('span', {}, 'Corners'), sidesInput, sidesOutput);
  const fillRow = el('label', { className: 'setting check' }, el('span', {}, 'Fill shapes'), el('input', {
    className: 'switch', type: 'checkbox', role: 'switch', checked: drawView.fill,
    onchange: (event) => { drawView.fill = event.target.checked; },
  }));
  const smallButton = (label, onclick) => el('button', { className: 'chip', type: 'button', onclick }, label);
  const selectedTools = el('div', { className: 'control' },
    el('span', { className: 'control-label' }, 'Selected line'),
    el('div', { className: 'chips' },
      smallButton('Duplicate', duplicateSelected),
      smallButton('To front', () => reorder((strokes) => strokes.length)),
      smallButton('To back', () => reorder(() => 0)),
      smallButton('Delete', deleteSelected)));

  const lookSlider = (label, key, max) => {
    const output = el('output', {}, String(work.config[key] ?? 0));
    const input = el('input', {
      type: 'range', min: 0, max, value: work.config[key] ?? 0,
      oninput: (event) => {
        work.config[key] = Number(event.target.value);
        if (key === 'glow' && !hasShape(work.config) && work.config.strokes.length) work.config.color = work.config.strokes[0].color;
        output.textContent = event.target.value;
        setFill(event.target);
        changed();
      },
    });
    setFill(input);
    return el('label', { className: 'field' }, el('span', {}, label), input, output);
  };

  const colorInput = el('input', {
    type: 'color', 'aria-label': 'Line color', value: drawView.color.toLowerCase(),
    oninput: (event) => {
      useColor(event.target.value.toUpperCase());
    },
  });
  const swatches = SWATCHES.map((color) => el('button', {
    className: 'swatch',
    type: 'button',
    title: color,
    'aria-label': `Draw in ${color}`,
    style: `background:${color}`,
    onclick: () => {
      useColor(color);
    },
  }));
  function useColor(color) {
    drawView.color = color;
    colorInput.value = color.toLowerCase();
    showSwatch();
    changeSelected((stroke) => { stroke.color = color; });
  }
  function showSwatch() {
    swatches.forEach((each, index) => each.setAttribute('aria-pressed', String(SWATCHES[index] === drawView.color)));
  }
  showSwatch();

  pageKeys.undo = () => stepBack(work.undo, work.redo);
  pageKeys.redo = () => stepBack(work.redo, work.undo);
  pageKeys.letter = (key) => {
    if (key === 'delete' || key === 'backspace') deleteSelected();
    if (DRAW_KEYS[key]) document.querySelector(`[data-choice="tool-${DRAW_KEYS[key]}"]`)?.click();
    if (key === '[' || key === ']') {
      widthInput.value = Number(widthInput.value) + (key === ']' ? 1 : -1);
      widthInput.dispatchEvent(new Event('input'));
    }
  };

  refresh();

  return [
    el('header', { className: 'page-head' },
      el('h1', { className: 'title' }, 'Draw'),
      el('div', { className: 'head-actions' },
        saveNote, undoButton, redoButton,
        el('button', { className: 'button', type: 'button', onclick: center }, 'Center'),
        el('button', { className: 'button', type: 'button', onclick: clear }, 'Clear'))),
    el('div', { className: 'editor draw' },
      el('section', { className: 'panel card', 'aria-label': 'Drawing' },
        stage,
        el('div', { className: 'stage-row' },
          el('div', {}, actualStage, el('p', { className: 'caption' }, 'Actual size')),
          el('div', {},
            choice('Zoom', DRAW_ZOOMS, 'zoom'),
            choice('Background', BACKDROPS, 'backdrop'),
            resetView))),
      el('section', { className: 'panel card', 'aria-label': 'Drawing tools' },
        el('label', { className: 'field-name control' }, el('span', {}, 'Name'), el('input', {
          className: 'text', type: 'text', maxLength: 40, value: work.name,
          oninput: (event) => {
            work.name = event.target.value;
            changed();
          },
        })),
        choice('Tool', DRAW_TOOLS, 'tool'),
        sidesRow,
        selectedTools,
        el('label', { className: 'field' }, el('span', {}, 'Line width'), widthInput, widthOutput),
        el('div', { className: 'control draw-colors' },
          el('span', { className: 'control-label' }, 'Color'),
          el('div', { className: 'swatches', role: 'group', 'aria-label': 'Quick colors' }, swatches),
          colorInput),
        choice('Line style', LINE_STYLES, 'style'),
        fillRow,
        choice('Mirror', MIRRORS, 'mirror'),
        el('div', { className: 'control' },
          el('span', { className: 'control-label' }, 'Change drawing'),
          el('div', { className: 'chips' },
            smallButton('Flip across', () => transform((x, y) => [-x, y])),
            smallButton('Flip down', () => transform((x, y) => [x, -y])),
            smallButton('Turn 45°', () => turnBy(45)),
            smallButton('Turn 90°', () => turnBy(90)),
            smallButton('Bigger', () => transform((x, y) => [x * GROW, y * GROW], GROW)),
            smallButton('Smaller', () => transform((x, y) => [x / GROW, y / GROW], 1 / GROW)))),
        el('label', { className: 'setting check' }, el('span', {}, 'Snap to pixels'), el('input', {
          className: 'switch', type: 'checkbox', role: 'switch', checked: drawView.snap,
          onchange: (event) => { drawView.snap = event.target.checked; },
        })),
        el('label', { className: 'setting check' }, el('span', {}, 'Smooth pen lines'), el('input', {
          className: 'switch', type: 'checkbox', role: 'switch', checked: drawView.smooth,
          onchange: (event) => { drawView.smooth = event.target.checked; },
        })),
        lookSlider('Outline', 'outline', 6),
        lookSlider('Glow', 'glow', 16),
        el('p', { className: 'tool-note' }, 'Hold Shift to keep a line straight. Scroll to zoom. Drag with the middle mouse button to move around. With a line selected, colors and width change that line.'),
        el('div', { className: 'actions' }, saveButton, useButton, editButton))),
  ];
}
