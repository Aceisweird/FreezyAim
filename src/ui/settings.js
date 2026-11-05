'use strict';

const TEXT_SIZES = [[90, 'Small'], [100, 'Normal'], [110, 'Large'], [125, 'Extra large']];
const SETTINGS_TABS = [['look', 'Appearance'], ['themes', 'Themes'], ['crosshair', 'Crosshair'], ['keys', 'Shortcuts', true], ['app', 'App']];
const THEMES = [
  ['fade', 'Color fade', 'Fades out from your accent color'],
  ['minimal', 'Minimal', 'Dark, with scopes at the edges'],
  ['', 'Default', 'The original Freezy Aim look'],
];
const MODIFIER_KEYS = ['Control', 'Shift', 'Alt', 'Meta'];
const ALT = 1;
const CTRL = 2;
const SHIFT = 4;
const FIRST_FUNCTION_KEY = 112; // F1
const LAST_FUNCTION_KEY = 135; // F24

let settingsTab = 'look';

function setting(label, description, ...controls) {
  return el('div', { className: controls.length > 1 ? 'setting' : 'setting simple' },
    el('div', {}, label, description ? el('small', {}, description) : null),
    ...controls);
}

function switchSetting(label, description, checked, change) {
  return el('label', { className: 'setting check' },
    el('div', {}, label, description ? el('small', {}, description) : null),
    el('input', {
      className: 'switch', type: 'checkbox', role: 'switch', checked,
      onchange: (event) => change(event.target.checked),
    }));
}

function lookSettings(refresh) {
  const theme = { ...state.theme };
  const warning = el('p', { className: 'warning', role: 'status' });
  const checkContrast = () => {
    warning.textContent = state.theme_contrast < 3
      ? 'These two colors are close, so selected items may be hard to see.'
      : '';
  };
  const saveTheme = debounce(async () => {
    if (await act('/api/theme', theme)) checkContrast();
  }, 100);

  const colorSetting = (label, description, key) => {
    const hex = el('input', {
      className: 'text', type: 'text', maxLength: 7, value: theme[key],
      'aria-label': `${label} hex code`,
      oninput: (event) => {
        const valid = /^#[0-9a-f]{6}$/i.test(event.target.value);
        event.target.setAttribute('aria-invalid', String(!valid));
        if (!valid) return;
        theme[key] = event.target.value.toUpperCase();
        swatch.value = theme[key].toLowerCase();
        saveTheme();
      },
    });
    const swatch = el('input', {
      type: 'color', value: theme[key].toLowerCase(), 'aria-label': label,
      oninput: (event) => {
        theme[key] = event.target.value.toUpperCase();
        hex.value = theme[key];
        hex.setAttribute('aria-invalid', 'false');
        saveTheme();
      },
    });
    return setting(label, description, swatch, hex);
  };

  const font = state.options.font || state.fonts[0];
  const size = state.options.text_size || 100;
  checkContrast();
  return [
    colorSetting('Main color', 'The background of the app', 'main'),
    colorSetting('Accent color', 'Gradients and selected items', 'accent'),
    warning,
    setting('Font', 'Used for all text except the name in the sidebar', el('select', {
      'aria-label': 'Font',
      onchange: (event) => act('/api/options', { font: event.target.value }),
    }, state.fonts.map((name) => el('option', {
      value: name, selected: name === font, style: `font-family:"${name}"`,
    }, name)))),
    setting('Text size', null, el('div', { className: 'chips', role: 'group', 'aria-label': 'Text size' },
      TEXT_SIZES.map(([value, text]) => el('button', {
        className: 'chip', type: 'button', 'aria-pressed': String(size === value),
        onclick: async () => {
          if (!(await act('/api/options', { text_size: value }))) return;
          refresh();
          gainColor(document.querySelector('.chip[aria-pressed="true"]'));
        },
      }, text)))),
    state.system_features ? switchSetting('Hide title bar', "Uses the app's own window buttons instead", !state.options.keep_title_bar,
      (on) => act('/api/options', { keep_title_bar: !on })) : null,
    switchSetting('Animations', 'Fades and movement around the app', !state.options.no_animations,
      (on) => act('/api/options', { no_animations: !on })),
    el('div', { className: 'actions' }, el('button', {
      className: 'button', type: 'button',
      onclick: async () => { if (await act('/api/theme', state.default_theme)) refresh(); },
    }, 'Reset colors')),
  ];
}

function nudgeButton(label, glyph, key, by, refresh) {
  const id = `nudge-${key}-${by}`;
  return el('button', {
    className: `button nudge ${glyph}`, type: 'button', 'aria-label': label, 'data-key': id,
    onclick: async () => {
      if (!(await act('/api/overlay', { [key]: state.overlay[key] + by }))) return;
      refresh();
      document.querySelector(`[data-key="${id}"]`)?.focus();
    },
  }, icon(ICONS.left));
}

function themeSettings(refresh) {
  const current = state.options.background ?? '';
  return el('div', { className: 'themes', role: 'group', 'aria-label': 'Themes' }, THEMES.map(([id, name, about]) => el('button', {
    className: 'theme',
    type: 'button',
    'aria-pressed': String(current === id),
    'data-key': `theme-${id}`,
    onclick: async () => {
      if (!(await act('/api/options', { background: id }))) return;
      refresh();
      document.querySelector(`[data-key="theme-${id}"]`)?.focus();
    },
  },
    el('span', { className: `theme-view ${id || 'plain'}` }),
    el('b', {}, name),
    el('small', {}, about))));
}

function crosshairSettings(refresh) {
  const monitors = Array.from({ length: Math.max(state.overlay.monitors, 1) }, (_, index) => el('option',
    { value: index, selected: index === state.overlay.screen },
    index === 0 ? 'Primary monitor' : `Monitor ${index + 1}`));
  return [
    switchSetting('Show crosshair', null, state.overlay.visible,
      (visible) => act('/api/overlay', { visible })),
    el('label', { className: 'setting simple' },
      el('span', {}, 'Monitor'),
      el('select', { onchange: (event) => act('/api/overlay', { screen: Number(event.target.value) }) }, monitors)),
    setting('Nudge', 'One pixel at a time', el('div', { className: 'nudges' },
      nudgeButton('Move left one pixel', 'left', 'offset_x', -1, refresh),
      nudgeButton('Move up one pixel', 'up', 'offset_y', -1, refresh),
      nudgeButton('Move down one pixel', 'down', 'offset_y', 1, refresh),
      nudgeButton('Move right one pixel', 'right', 'offset_x', 1, refresh))),
    el('div', { className: 'actions' }, el('button', {
      className: 'button', type: 'button',
      onclick: async () => { if (await act('/api/overlay', { offset_x: 0, offset_y: 0 })) refresh(); },
    }, 'Center crosshair')),
  ];
}

function hotkeyFrom(event) {
  const modifiers = (event.altKey ? ALT : 0) | (event.ctrlKey ? CTRL : 0) | (event.shiftKey ? SHIFT : 0);
  const functionKey = event.keyCode >= FIRST_FUNCTION_KEY && event.keyCode <= LAST_FUNCTION_KEY;
  if (!modifiers && !functionKey) return null;
  const name = event.key === ' ' ? 'Space' : event.key.length === 1 ? event.key.toUpperCase() : event.key;
  const parts = [event.ctrlKey && 'Ctrl', event.altKey && 'Alt', event.shiftKey && 'Shift', name].filter(Boolean);
  return { key: event.keyCode, modifiers, label: parts.join('+') };
}

function shortcutSetting(label, description, key, refresh) {
  const current = () => state.options[key].label || 'Not set';
  const button = el('button', { className: 'button shortcut', type: 'button', 'aria-label': `${label} shortcut` }, current());
  const listening = () => button.classList.contains('listening');
  const stop = () => {
    button.classList.remove('listening');
    button.textContent = current();
  };
  button.addEventListener('click', () => {
    button.classList.add('listening');
    button.textContent = 'Press keys';
  });
  button.addEventListener('blur', stop);
  button.addEventListener('keydown', async (event) => {
    if (!listening()) return;
    event.preventDefault();
    if (event.key === 'Escape') return stop();
    if (MODIFIER_KEYS.includes(event.key)) return;
    const hotkey = hotkeyFrom(event);
    if (!hotkey) {
      showNotice('Add Ctrl, Alt or Shift to that key, or use a function key like F8.');
      return;
    }
    if (await act('/api/options', { [key]: hotkey })) refresh();
  });
  const clear = el('button', {
    className: 'button', type: 'button', 'aria-label': `Clear ${label} shortcut`,
    disabled: !state.options[key].key,
    onclick: async () => { if (await act('/api/options', { [key]: { key: 0, modifiers: 0, label: '' } })) refresh(); },
  }, 'Clear');
  return setting(label, description, el('div', { className: 'shortcut-actions' }, button, clear));
}

function keySettings(refresh) {
  return [
    el('p', { className: 'lead' }, 'Shortcuts work everywhere, including while you play.'),
    shortcutSetting('Show or hide crosshair', null, 'toggle_key', refresh),
    shortcutSetting('Next favorite', 'Switches to your next favorite crosshair', 'next_key', refresh),
  ];
}

function downloadBackup() {
  const crosshairs = state.crosshairs.map(({ name, config }) => ({ name, config }));
  const data = new Blob([JSON.stringify(crosshairs, null, 2)], { type: 'application/json' });
  const link = el('a', { href: URL.createObjectURL(data), download: 'freezy-aim-crosshairs.json' });
  link.click();
  setTimeout(() => URL.revokeObjectURL(link.href), 10000);
}

async function restoreBackup(file) {
  let items = null;
  try {
    items = JSON.parse(await file.text());
  } catch {
    // Handled below, because a file can also read fine and still not be a backup.
  }
  if (!Array.isArray(items) || !items.every((item) => item?.config)) {
    flash("That isn't a Freezy Aim backup.");
    return;
  }
  let added = 0;
  for (const { name, config } of items) {
    if (await act('/api/crosshair', { name, config })) added += 1;
  }
  renderNav();
  flash(`Added ${added} ${added === 1 ? 'crosshair' : 'crosshairs'}.`);
}

function appSettings(refresh) {
  const picker = el('input', {
    type: 'file', accept: '.json', hidden: true, 'aria-label': 'Backup file',
    onchange: (event) => {
      if (event.target.files[0]) restoreBackup(event.target.files[0]);
      event.target.value = '';
    },
  });
  return [
    state.system_features ? switchSetting('Start with Windows', 'Runs Freezy Aim when you sign in', state.overlay.starts_with_windows,
      (on) => act('/api/options', { start_with_windows: on })) : null,
    switchSetting('Open this window at startup', 'Turn off to start quietly in the tray', !state.options.start_in_tray,
      (on) => act('/api/options', { start_in_tray: !on })),
    setting('Version', null, el('span', { className: 'value' }, state.version)),
    setting('Your data', 'Saved crosshairs, settings and the error log', el('button', {
      className: 'button', type: 'button',
      onclick: () => act('/api/open-folder', {}),
    }, 'Open folder')),
    setting('Keyboard shortcuts', 'See what you can do from the keyboard', el('button', {
      className: 'button', type: 'button', onclick: showShortcuts,
    }, 'Show list')),
    setting('Your crosshairs', 'Keep a copy, or bring one back after reinstalling', el('div', { className: 'shortcut-actions' },
      picker,
      el('button', { className: 'button', type: 'button', onclick: downloadBackup }, 'Back up'),
      el('button', { className: 'button', type: 'button', onclick: () => picker.click() }, 'Restore'))),
    el('div', { className: 'actions' }, el('button', {
      className: 'button', type: 'button',
      onclick: async () => {
        if (!(await askFirst('Restore default settings?', 'Your saved crosshairs and favorites are kept.', 'Restore'))) return;
        if (await act('/api/restore', {})) refresh();
      },
    }, 'Restore default settings')),
  ];
}

const SETTINGS_PANELS = { look: lookSettings, themes: themeSettings, crosshair: crosshairSettings, keys: keySettings, app: appSettings };

function settingsPage() {
  const body = el('div', { className: 'tab-body' });
  const tabs = SETTINGS_TABS
    .filter(([, , needsSystem]) => !needsSystem || state.system_features)
    .map(([id, text]) => el('button', {
      className: 'tab',
      type: 'button',
      role: 'tab',
      'data-tab': id,
      onclick: () => show(id),
    }, text));
  const show = (tab) => {
    const step = tabStep(tabs, settingsTab, tab);
    settingsTab = tab;
    for (const button of tabs) button.setAttribute('aria-selected', String(button.dataset.tab === tab));
    swapPanel(body, step, () => body.replaceChildren(...[SETTINGS_PANELS[tab](() => show(tab))].flat(2).filter(Boolean)));
    if (step) gainColor(tabs.find((button) => button.dataset.tab === tab));
  };
  show(settingsTab);
  return [
    el('h1', { className: 'title' }, 'Settings'),
    el('section', { className: 'panel card settings' },
      el('div', { className: 'tabs', role: 'tablist', 'aria-label': 'Settings' }, tabs),
      body),
  ];
}
