'use strict';

// The Crosshair Editor. Each change saves itself a moment later, and if the
// crosshair is in use, the overlay updates as you work.

const BACKDROPS = [['dark', 'Dark'], ['grey', 'Grey'], ['light', 'Light'], ['sky', 'Sky'], ['grass', 'Grass']];
const ZOOMS = [[1, '1x'], [2, '2x'], [3, '3x'], [4, '4x']];
const ARMS = [['t', 'Top'], ['r', 'Right'], ['b', 'Bottom'], ['l', 'Left']];
const DOT_SHAPES = [['', 'Square'], ['round', 'Round'], ['diamond', 'Diamond']];
const SWATCHES = ['#FFFFFF', '#00FF00', '#00FFFF', '#FFFF00', '#FF2020', '#FF2D95', '#FF8000', '#000000'];
const FRAMES = [
  ['', 'None'], ['circle', 'Circle'], ['square', 'Square'], ['diamond', 'Diamond'],
  ['triangle', 'Triangle'], ['hexagon', 'Hexagon'], ['chevron', 'Chevron'], ['brackets', 'Brackets'],
];

// These carry over when you switch bases, so a new shape keeps your colours.
const LOOK_SETTINGS = ['color', 'outline_color', 'outline', 'opacity', 'glow', 'dot_color', 'frame_color', 'strokes', 'merged', 'picture', 'picture_size', 'picture_x', 'picture_y'];
const IMAGE_SIDES = [128, 96, 64, 48]; // Stored sizes to try, largest first.
const MAX_IMAGE_DATA = 200000;
const MAX_IMAGE_MOVE = 200;
const MAX_MERGED = 3;

const editorView = { tab: 'base', zoom: 3, backdrop: 'dark' };

// Set while an edit is waiting to be saved, so leaving the page can finish it first.
let saveEditorNow = null;

function setFill(input) {
  const share = (Number(input.value) - Number(input.min)) / (Number(input.max) - Number(input.min));
  input.style.setProperty('--fill', `${share * 100}%`);
}

// Pictures are shrunk here, so any format the browser reads becomes a small PNG.
async function pictureData(file) {
  const bitmap = await createImageBitmap(file);
  for (const side of IMAGE_SIDES) {
    const fit = Math.min(side / Math.max(bitmap.width, bitmap.height), 1);
    const canvas = new OffscreenCanvas(Math.max(Math.round(bitmap.width * fit), 1), Math.max(Math.round(bitmap.height * fit), 1));
    const context = canvas.getContext('2d');
    context.imageSmoothingQuality = 'high';
    context.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
    const blob = await canvas.convertToBlob({ type: 'image/png' });
    const data = toBase64(new Uint8Array(await blob.arrayBuffer()));
    if (data.length <= MAX_IMAGE_DATA) return data;
  }
  return null;
}

// Some settings only show once another is set: arms need both a length and a thickness.
// A crosshair that's only a dot or a picture has neither, so this fills in the missing one.
function wakeShape(config, key) {
  let woke = false;
  const fill = (name, value) => {
    if (!config[name]) {
      config[name] = value;
      woke = true;
    }
  };
  if (['length', 'gap', 'thickness'].includes(key)) {
    if (config.hide_arms?.length === 4) {
      delete config.hide_arms;
      woke = true;
    }
    fill('length', 8);
    fill('thickness', 2);
  }
  if (key === 'dot_shape') fill('dot_size', 4);
  return woke;
}

const hasArmsSet = (config) => config.length > 0 && config.thickness > 0;

// With no arms, dot or frame, a drawing's lines are all there is to color.
function setMainColor(config, color) {
  config.color = color;
  const shape = hasArmsSet(config) || config.dot_size > 0 || config.frame;
  if (!shape) for (const stroke of config.strokes ?? []) stroke.color = color;
}

function pickGroups(groups, onPick) {
  return groups.filter(([, items]) => items.length).map(([group, items]) => el('section', { className: 'base-group', 'aria-label': group },
    el('h3', {}, group),
    el('div', { className: 'bases' }, items.map((item) => el('button', {
      className: 'base',
      type: 'button',
      'data-key': `base-${item.id}`,
      onclick: () => onPick(item),
    },
      el('span', { className: 'base-view' }, preview(item.config)),
      el('span', { className: 'base-name' }, item.name))))));
}

function baseList(onPick) {
  return pickGroups(state.base_groups.map((group) => [group, state.bases.filter((base) => base.category === group)]), onPick);
}

function editorStart() {
  const create = async (base) => {
    const reply = await act('/api/crosshair', { name: base.name, config: base.config });
    if (reply) location.hash = `editor/${reply.id}`;
  };
  return [
    el('h1', { className: 'title' }, 'Crosshair editor'),
    el('section', { className: 'panel card start' },
      el('h2', {}, 'Choose a base'),
      el('p', { className: 'lead' }, 'Pick a starting shape, then make it your own.'),
      el('div', { className: 'start-draw' },
        el('span', {}, 'Rather start from nothing?'),
        el('button', { className: 'button', type: 'button', onclick: () => { location.hash = 'editor/draw'; } }, 'Draw your own')),
      baseList(create)),
  ];
}

function editorWorkspace(item) {
  const draft = structuredClone(item);
  const undoStack = [];
  const redoStack = [];
  let lastEdit = 0;

  const large = el('img', { alt: `Enlarged preview of ${item.name}` });
  const actual = el('img', { alt: `Actual size preview of ${item.name}` });
  const stage = el('figure', { className: 'stage', onpointerdown: startImageDrag, onpointermove: dragImage, onpointerup: endImageDrag, onpointercancel: endImageDrag }, large);
  const actualStage = el('figure', { className: 'stage actual' }, actual);
  const tabBody = el('div', { className: 'tab-body' });
  const undoButton = el('button', { className: 'button', type: 'button', onclick: () => restore(undoStack, redoStack) }, 'Undo');
  const redoButton = el('button', { className: 'button', type: 'button', onclick: () => restore(redoStack, undoStack) }, 'Redo');
  const useButton = el('button', { type: 'button', onclick: useCrosshair });
  const saveState = el('span', { className: 'save-state', role: 'status' }, 'Saves automatically');
  const nameInput = el('input', {
    className: 'text', type: 'text', maxLength: 40, value: draft.name,
    oninput: (event) => edit(() => { draft.name = event.target.value; }),
  });

  function refresh() {
    large.srcset = previewSource(draft.config, editorView.zoom);
    actual.srcset = previewSource(draft.config, 1);
    stage.className = `stage ${editorView.backdrop} ${draft.config.picture && editorView.tab === 'image' ? 'movable' : ''}`;
    actualStage.className = `stage actual ${editorView.backdrop}`;
    undoButton.disabled = undoStack.length === 0;
    redoButton.disabled = redoStack.length === 0;
    const inUse = state.active === draft.id;
    useButton.className = `button ${inUse ? 'good' : 'primary'}`;
    useButton.textContent = inUse ? 'In use' : 'Use crosshair';
  }

  async function saveDraft() {
    saveEditorNow = null;
    const reply = await act('/api/crosshair', draft);
    if (reply) renderNav();
    saveState.textContent = reply ? 'Saved' : 'Not saved';
    return reply;
  }

  const saveLater = debounce(() => saveEditorNow?.(), 120);
  const saveSoon = () => {
    saveState.textContent = 'Saving...';
    saveLater();
  };

  // Quick changes in a row, like dragging a slider, count as one step for Undo.
  function edit(change) {
    const now = Date.now();
    if (now - lastEdit > 700) {
      undoStack.push(JSON.stringify(draft));
      redoStack.length = 0;
    }
    lastEdit = now;
    change(draft.config);
    refresh();
    saveEditorNow = saveDraft;
    saveSoon();
  }

  // Picking something is always its own step for Undo, however quickly it follows.
  function step(change) {
    lastEdit = 0;
    edit(change);
    lastEdit = 0;
  }

  function restore(from, to) {
    if (!from.length) return;
    to.push(JSON.stringify(draft));
    Object.assign(draft, JSON.parse(from.pop()));
    lastEdit = 0;
    nameInput.value = draft.name;
    showTab(editorView.tab);
    refresh();
    saveEditorNow = saveDraft;
    saveSoon();
  }

  async function useCrosshair() {
    await saveEditorNow?.();
    if (!(await act('/api/use', { id: draft.id }))) return;
    refresh();
    gainColor(useButton);
    lockOn(stage);
  }

  function applyBase(base) {
    lastEdit = 0;
    edit((config) => {
      const look = Object.fromEntries(LOOK_SETTINGS
        .filter((key) => key in config)
        .map((key) => [key, config[key]]));
      for (const key of Object.keys(config)) delete config[key];
      Object.assign(config, base.config, look);
    });
    showTab('base');
  }

  // Controls

  const slider = (label, key, { min = 0, max, step = 1, format = String }, partOf = (config) => config) => {
    const value = partOf(draft.config)[key] ?? 0;
    const output = el('output', {}, format(value));
    let woke = false;
    const input = el('input', {
      type: 'range', min, max, step, value,
      oninput: (event) => {
        const next = Number(event.target.value);
        output.textContent = format(next);
        setFill(event.target);
        edit((config) => {
          partOf(config)[key] = next;
          woke = wakeShape(partOf(config), key) || woke;
        });
      },
      onchange: () => {
        if (woke) showTab(editorView.tab); // Shows the other values that changed.
        woke = false;
      },
    });
    setFill(input);
    return el('label', { className: 'field' }, el('span', {}, label), input, output);
  };

  const toggle = (label, key) => el('label', { className: 'setting check' },
    el('span', {}, label),
    el('input', {
      className: 'switch', type: 'checkbox', role: 'switch', checked: Boolean(draft.config[key]),
      onchange: (event) => {
        edit((config) => {
          config[key] = event.target.checked;
          if (event.target.checked) wakeShape(config, key);
        });
        showTab(editorView.tab);
      },
    }));

  const chips = (label, options, isOn, pick) => el('div', { className: 'control' },
    el('span', { className: 'control-label' }, label),
    el('div', { className: 'chips', role: 'group', 'aria-label': label }, options.map(([value, text]) => el('button', {
      className: 'chip',
      type: 'button',
      'aria-pressed': String(isOn(value)),
      'data-key': `${label}-${value}`,
      onclick: () => {
        lastEdit = 0;
        pick(value);
        lastEdit = 0;
        showTab(editorView.tab);
        const chosen = tabBody.querySelector(`[data-key="${CSS.escape(`${label}-${value}`)}"]`);
        chosen?.focus();
        gainColor(chosen);
      },
    }, text))));

  const viewChips = (label, options, key, changed) => {
    const buttons = options.map(([value, text]) => el('button', {
      className: 'chip',
      type: 'button',
      'aria-pressed': String(editorView[key] === value),
      onclick: (event) => {
        const before = editorView[key];
        editorView[key] = value;
        for (const button of buttons) button.setAttribute('aria-pressed', String(button === event.currentTarget));
        gainColor(event.currentTarget);
        refresh();
        changed(before);
      },
    }, text));
    return el('div', { className: 'control' },
      el('span', { className: 'control-label' }, label),
      el('div', { className: 'chips', role: 'group', 'aria-label': label }, buttons));
  };

  // The new zoom grows out of the old one once it has been drawn.
  const zoomFrom = async (before) => {
    await large.decode().catch(() => {});
    if (!calm()) large.animate({ scale: [before / editorView.zoom, 1] }, { duration: 500, easing: SPRING });
  };

  const fadeBackdrop = () => {
    if (calm()) return;
    for (const each of [stage, actualStage]) each.animate({ opacity: [0.35, 1] }, { duration: 320, easing: 'ease-out' });
  };

  const colorRow = (label, key, canMatch = false) => {
    const matching = canMatch && !draft.config[key];
    return el('div', { className: 'color-row' },
      el('span', {}, label),
      el('input', {
        type: 'color',
        'aria-label': label,
        value: (draft.config[key] || draft.config.color).toLowerCase(),
        disabled: matching,
        oninput: (event) => edit((config) => {
          if (key === 'color') setMainColor(config, event.target.value.toUpperCase());
          else config[key] = event.target.value.toUpperCase();
        }),
      }),
      canMatch ? el('label', { className: 'match' },
        el('input', {
          className: 'switch', type: 'checkbox', role: 'switch', checked: matching,
          'aria-label': `${label} matches crosshair color`,
          onchange: (event) => {
            edit((config) => {
              if (event.target.checked) delete config[key];
              else config[key] = config.color;
            });
            showTab('color');
          },
        }), 'Match') : el('span', {}));
  };

  const hiddenArms = () => draft.config.hide_arms ?? '';

  function toggleArm(letter) {
    edit((config) => {
      const hidden = hiddenArms();
      const next = hidden.includes(letter) ? hidden.replace(letter, '') : hidden + letter;
      if (next) config.hide_arms = next;
      else delete config.hide_arms;
    });
  }

  function chooseFrame(frame) {
    edit((config) => {
      if (frame) {
        config.frame = frame;
        config.frame_size = config.frame_size || 18;
        config.frame_thickness = config.frame_thickness || 2;
      } else {
        for (const key of ['frame', 'frame_size', 'frame_thickness', 'frame_gaps', 'frame_color']) delete config[key];
      }
    });
  }

  const degrees = (value) => `${value}°`;
  const percent = (value) => `${Math.round(value * 100)}%`;

  // An imported picture replaces the crosshair's shape. Undo brings the shape back.
  const picker = el('input', {
    type: 'file', accept: 'image/*', hidden: true, 'aria-label': 'Picture to import',
    onchange: async (event) => {
      const [file] = event.target.files;
      event.target.value = '';
      if (file) await importImage(file);
    },
  });

  async function importImage(file) {
    let data = null;
    try {
      data = await pictureData(file);
    } catch {
      flash("That picture couldn't be opened. Try a PNG or JPG.");
      return;
    }
    if (!data) {
      flash('That picture has too much detail. Try a simpler one.');
      return;
    }
    step((config) => {
      for (const key of ['frame', 'frame_size', 'frame_thickness', 'frame_gaps', 'frame_color', 'dot_shape', 'dot_color', 'glow', 'picture_x', 'picture_y']) delete config[key];
      Object.assign(config, { picture: data, picture_size: config.picture_size || 64, length: 0, gap: 0, thickness: 0, dot_size: 0, outline: 0 });
    });
    showTab('image');
  }

  function moveImage(x, y) {
    edit((config) => {
      config.picture_x = Math.min(Math.max(x, -MAX_IMAGE_MOVE), MAX_IMAGE_MOVE);
      config.picture_y = Math.min(Math.max(y, -MAX_IMAGE_MOVE), MAX_IMAGE_MOVE);
    });
  }

  function removeImage() {
    step((config) => {
      for (const key of ['picture', 'picture_size', 'picture_x', 'picture_y']) delete config[key];
    });
    showTab('image');
  }

  // Dragging the picture in the preview moves it.
  let imageDrag = null;

  function startImageDrag(event) {
    if (event.button !== 0 || !draft.config.picture || editorView.tab !== 'image') return;
    stage.setPointerCapture(event.pointerId);
    const fit = large.naturalWidth ? large.clientWidth / large.naturalWidth : 1;
    imageDrag = { x: event.clientX, y: event.clientY, imageX: draft.config.picture_x ?? 0, imageY: draft.config.picture_y ?? 0, scale: editorView.zoom * fit };
    lastEdit = 0;
  }

  function dragImage(event) {
    if (!imageDrag) return;
    const x = imageDrag.imageX + Math.round((event.clientX - imageDrag.x) / imageDrag.scale);
    const y = imageDrag.imageY + Math.round((event.clientY - imageDrag.y) / imageDrag.scale);
    if (x !== (draft.config.picture_x ?? 0) || y !== (draft.config.picture_y ?? 0)) moveImage(x, y);
  }

  function endImageDrag() {
    imageDrag = null;
    lastEdit = 0;
  }

  const imageNudge = (label, glyph, moveX, moveY) => el('button', {
    className: `button nudge ${glyph}`, type: 'button', 'aria-label': label,
    onclick: () => moveImage((draft.config.picture_x ?? 0) + moveX, (draft.config.picture_y ?? 0) + moveY),
  }, icon(ICONS.left));

  // Merging stacks a copy of another crosshair on this one.
  function mergeWith(item) {
    step((config) => {
      const added = [{ ...item.config, merged: undefined, offset_x: 0, offset_y: 0, screen: 0 }, ...(item.config.merged ?? [])];
      config.merged = [...(config.merged ?? []), ...added].slice(0, MAX_MERGED);
    });
    showTab('merge');
    page.scrollTo({ top: 0 }); // The new layer's controls are at the top of the tab.
  }

  function unmerge(index) {
    step((config) => {
      config.merged.splice(index, 1);
      if (!config.merged.length) delete config.merged;
    });
    showTab('merge');
  }

  const mergedRow = (layer, index) => {
    const partOf = (config) => config.merged[index];
    return el('div', { className: 'merged' },
      el('div', {},
        el('span', { className: 'base-view' }, preview(layer)),
        el('button', { className: 'button', type: 'button', onclick: () => unmerge(index) }, 'Remove')),
      el('div', {},
        chips(`Layer ${index + 1}`, [[false, 'On top'], [true, 'Behind']], (behind) => Boolean(layer.behind) === behind, (behind) => step((config) => {
          if (behind) config.merged[index].behind = true;
          else delete config.merged[index].behind;
        })),
        slider('Opacity', 'opacity', { max: 1, step: 0.05, format: percent }, partOf),
        slider('Move right', 'offset_x', { min: -40, max: 40 }, partOf),
        slider('Move down', 'offset_y', { min: -40, max: 40 }, partOf)));
  };

  function mergeChoices() {
    const list = el('div', {});
    const fill = (text = '') => {
      const wanted = text.trim().toLowerCase();
      const named = (item) => item.name.toLowerCase().includes(wanted);
      list.replaceChildren(...pickGroups([
        ['My crosshairs', state.crosshairs.filter((item) => item.id !== draft.id && named(item))],
        ...state.categories.map((category) => [category, state.presets.filter((item) => item.category === category && named(item))]),
      ], mergeWith));
    };
    fill();
    return [
      el('input', {
        className: 'text search merge-search', type: 'search', placeholder: 'Search', 'aria-label': 'Search crosshairs to merge',
        oninput: (event) => fill(event.target.value),
      }),
      list,
    ];
  }

  const panels = {
    base: () => [
      el('p', { className: 'lead' }, 'Switching bases keeps your colors.'),
      baseList(applyBase),
    ],
    arms: () => [
      chips('Show arms', ARMS, (letter) => !hiddenArms().includes(letter), toggleArm),
      slider('Length', 'length', { max: 60 }),
      slider('Gap', 'gap', { max: 40 }),
      slider('Thickness', 'thickness', { max: 20 }),
      slider('Rotation', 'rotation', { max: 345, step: 15, format: degrees }),
      toggle('Rounded ends', 'round_ends'),
    ],
    frame: () => [
      chips('Style', FRAMES, (frame) => (draft.config.frame ?? '') === frame, chooseFrame),
      draft.config.frame ? [
        slider('Size', 'frame_size', { min: 2, max: 80 }),
        slider('Line width', 'frame_thickness', { min: 1, max: 12 }),
        slider('Gaps', 'frame_gaps', { max: 8 }),
        slider('Rotation', 'rotation', { max: 345, step: 15, format: degrees }),
      ] : null,
    ],
    dot: () => [
      slider('Size', 'dot_size', { max: 20 }),
      chips('Shape', DOT_SHAPES, (shape) => (draft.config.dot_shape ?? '') === shape, (shape) => edit((config) => {
        if (shape) {
          config.dot_shape = shape;
          wakeShape(config, 'dot_shape');
        } else delete config.dot_shape;
      })),
    ],
    color: () => [
      el('div', { className: 'control' },
        el('span', { className: 'control-label' }, 'Quick colors'),
        el('div', { className: 'swatches', role: 'group', 'aria-label': 'Quick colors' }, SWATCHES.map((color) => el('button', {
          className: 'swatch',
          type: 'button',
          title: color,
          'aria-label': `Use ${color}`,
          style: `background:${color}`,
          onclick: () => { edit((config) => setMainColor(config, color)); showTab('color'); },
        })))),
      draft.config.picture ? el('p', { className: 'lead' }, 'Pictures keep their own colors. Crosshair color tints their glow.') : null,
      (draft.config.strokes ?? []).length ? el('div', { className: 'color-row' },
        el('span', {}, 'Drawn lines'),
        el('input', {
          type: 'color', 'aria-label': 'Drawn lines',
          value: draft.config.strokes[0].color.toLowerCase(),
          oninput: (event) => edit((config) => {
            for (const stroke of config.strokes) stroke.color = event.target.value.toUpperCase();
          }),
        }),
        el('span', {})) : null,
      colorRow('Crosshair', 'color'),
      colorRow('Frame', 'frame_color', true),
      colorRow('Dot', 'dot_color', true),
      colorRow('Outline', 'outline_color'),
      el('div', { className: 'divider' }),
      slider('Outline', 'outline', { max: 6 }),
      slider('Opacity', 'opacity', { max: 1, step: 0.05, format: percent }),
      slider('Glow', 'glow', { max: 16 }),
    ],
    image: () => {
      const importButton = (text) => el('button', { className: 'button', type: 'button', onclick: () => picker.click() }, text);
      if (!draft.config.picture) {
        return [
          el('p', { className: 'lead' }, 'Use a picture as your crosshair. Pictures with a see-through background work best.'),
          el('div', { className: 'actions' }, importButton('Import image')),
        ];
      }
      return [
        el('p', { className: 'lead' }, 'Drag the picture in the preview to move it.'),
        slider('Size', 'picture_size', { min: 8, max: 200 }),
        slider('Rotation', 'rotation', { max: 345, step: 15, format: degrees }),
        el('div', { className: 'setting simple' },
          el('span', {}, 'Nudge'),
          el('div', { className: 'nudges' },
            imageNudge('Move the picture left', 'left', -1, 0),
            imageNudge('Move the picture up', 'up', 0, -1),
            imageNudge('Move the picture down', 'down', 0, 1),
            imageNudge('Move the picture right', 'right', 1, 0))),
        el('div', { className: 'actions' },
          el('button', { className: 'button', type: 'button', onclick: () => { lastEdit = 0; moveImage(0, 0); lastEdit = 0; } }, 'Center image'),
          importButton('Replace image'),
          el('button', { className: 'button', type: 'button', onclick: removeImage }, 'Remove image')),
      ];
    },
    merge: () => {
      const merged = draft.config.merged ?? [];
      return [
        el('p', { className: 'lead' }, 'Pick a crosshair to stack on this one. Each keeps its own colors.'),
        merged.map(mergedRow),
        merged.length < MAX_MERGED
          ? mergeChoices()
          : el('p', { className: 'lead' }, `That's the limit of ${MAX_MERGED}. Remove one to merge another.`),
      ];
    },
  };

  const tabs = ['Base', 'Arms', 'Frame', 'Dot', 'Color', 'Image', 'Merge'].map((text) => el('button', {
    className: 'tab',
    type: 'button',
    role: 'tab',
    'data-tab': text.toLowerCase(),
    onclick: () => showTab(text.toLowerCase()),
  }, text));

  function showTab(wanted) {
    const step = tabStep(tabs, editorView.tab, wanted);
    editorView.tab = wanted;
    for (const tab of tabs) tab.setAttribute('aria-selected', String(tab.dataset.tab === wanted));
    swapPanel(tabBody, step, () => tabBody.replaceChildren(...[panels[wanted]()].flat(3).filter(Boolean)));
    stage.classList.toggle('movable', Boolean(draft.config.picture) && wanted === 'image');
    if (step) gainColor(tabs.find((tab) => tab.dataset.tab === wanted));
  }

  pageKeys.undo = () => restore(undoStack, redoStack);
  pageKeys.redo = () => restore(redoStack, undoStack);

  showTab(editorView.tab);
  refresh();

  return [
    el('header', { className: 'page-head' },
      el('h1', { className: 'title' }, 'Crosshair editor'),
      el('div', { className: 'head-actions' }, saveState, undoButton, redoButton)),
    el('div', { className: 'editor' },
      el('section', { className: 'panel card', 'aria-label': `Preview of ${item.name}` },
        el('label', { className: 'field-name' }, el('span', {}, 'Name'), nameInput),
        stage,
        el('div', { className: 'stage-row' },
          el('div', {}, actualStage, el('p', { className: 'caption' }, 'Actual size')),
          el('div', {},
            viewChips('Zoom', ZOOMS, 'zoom', zoomFrom),
            viewChips('Background', BACKDROPS, 'backdrop', fadeBackdrop))),
        el('div', { className: 'actions' },
          useButton,
          el('button', {
            className: 'button',
            type: 'button',
            onclick: async () => {
              await saveEditorNow?.();
              const reply = await act('/api/copy', { id: draft.id });
              if (reply) location.hash = `editor/${reply.id}`;
            },
          }, 'Duplicate'),
          el('button', {
            className: 'button',
            type: 'button',
            onclick: () => { location.hash = `editor/draw/${draft.id}`; },
          }, 'Draw on it'),
          el('button', {
            className: 'button',
            type: 'button',
            onclick: async () => {
              await saveEditorNow?.();
              copyCode(draft);
            },
          }, 'Copy share code'),
          el('button', {
            className: 'button',
            type: 'button',
            onclick: async () => {
              if (!(await askFirst(`Delete ${draft.name}?`, "This can't be undone.", 'Delete'))) return;
              saveEditorNow = null;
              if (await act('/api/delete', { id: draft.id })) location.hash = 'mine';
            },
          }, 'Delete'))),
      el('section', { className: 'panel card options', 'aria-label': 'Crosshair options' },
        el('div', { className: 'tabs', role: 'tablist', 'aria-label': 'Crosshair options' }, tabs),
        tabBody,
        picker)),
  ];
}

function editorPage(id) {
  if (id === 'draw' || id.startsWith('draw/')) return drawPage(id.slice(5));
  const item = state.crosshairs.find((each) => each.id === id);
  return item ? editorWorkspace(item) : editorStart();
}
