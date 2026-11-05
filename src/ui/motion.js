'use strict';

// How Freezy Aim moves. Everything behaves like a sight settling on a target: it
// gets there fast, drifts a touch past the spot, then locks. Motion only answers
// what you do, so nothing runs and no power is used while the app sits open.

let state = null; // What the app is showing. Set up here because this script loads first.

const EASE = 'cubic-bezier(.22,.8,.24,1)';
const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)');

function curve(name) {
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return CSS.supports('transition-timing-function', value) ? value : EASE;
}

const SPRING = curve('--spring');
const POP = curve('--pop');

function debounce(run, wait) {
  let timer;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => run(...args), wait);
  };
}

function calm() {
  return reducedMotion.matches || Boolean(state?.options.no_animations);
}

// A newly selected button fills with color from the left, then settles.
function gainColor(button) {
  if (!button || calm()) return;
  button.animate({ backgroundSize: ['0% 100%, auto', '100% 100%, auto'] }, { duration: 420, easing: EASE });
  button.animate({ scale: [0.94, 1] }, { duration: 480, easing: POP });
}

// A ring closes on whatever was just chosen.
function lockOn(target) {
  if (!target || calm()) return;
  const ring = target.appendChild(el('span', { className: 'ping', 'aria-hidden': 'true' }));
  const closing = ring.animate({ scale: [2.3, 1], opacity: [0, 0.95, 0] }, { duration: 540, easing: EASE });
  closing.onfinish = closing.oncancel = () => ring.remove();
}

// Pages swap like a scope pulling focus. The new page animates live, so anything you
// click or draw while it settles still counts.
let leavingPage = null;

async function changePage(update) {
  if (calm()) {
    update();
    return;
  }
  leavingPage?.cancel();
  const leaving = page.animate(
    { opacity: [1, 0], filter: ['blur(0)', 'blur(7px)'], scale: [1, 0.985] },
    { duration: 120, easing: 'ease-in', fill: 'forwards' },
  );
  leavingPage = leaving;
  await leaving.finished.catch(() => {});
  if (leaving !== leavingPage) return; // A newer page change took over.
  leavingPage = null;
  update();
  leaving.cancel();
  page.animate({ opacity: [0, 1], filter: ['blur(7px)', 'blur(0)'], scale: [1.015, 1] }, { duration: 320, easing: EASE });
}

// Crosshairs arrive one after another, starting from the top of the page.
function arrive(wait = 0) {
  if (calm()) return;
  let order = 0;
  for (const item of page.querySelectorAll('.tile, .new-tile, .base')) {
    if (item.getBoundingClientRect().top > innerHeight) break;
    item.animate({ opacity: [0, 1], scale: [0.88, 1] }, {
      duration: 500, delay: wait + Math.min(order++, 16) * 14, easing: SPRING, fill: 'backwards',
    });
  }
}

function tabStep(tabs, from, to) {
  const place = (id) => tabs.findIndex((tab) => tab.dataset.tab === id);
  return Math.sign(place(to) - place(from));
}

// Swaps a panel's contents. The new tab comes in from the side it sits on, and the
// card eases to its new height.
function swapPanel(body, step, fill) {
  if (!step || calm()) {
    fill();
    return;
  }
  const card = body.parentElement;
  const before = card.offsetHeight;
  fill();
  const after = card.offsetHeight;
  if (after !== before) {
    card.animate({ height: [`${before}px`, `${after}px`], overflow: ['hidden', 'hidden'] }, { duration: 360, easing: EASE });
  }
  [...body.children].forEach((child, index) => {
    child.animate({ opacity: [0, 1], translate: [`${step * 20}px 0`, '0 0'] }, {
      duration: 440, delay: Math.min(index, 8) * 18, easing: SPRING, fill: 'backwards',
    });
  });
}

// The wheel eases a rail along instead of moving it in jumps.
const glides = new WeakMap();

function glide(track, distance) {
  if (calm()) {
    track.scrollBy({ left: distance });
    return;
  }
  const moving = glides.has(track);
  const start = moving ? glides.get(track) : track.scrollLeft;
  glides.set(track, Math.min(Math.max(start + distance, 0), track.scrollWidth - track.clientWidth));
  if (moving) return;
  let place = track.scrollLeft;
  let last = performance.now();
  track.style.scrollBehavior = 'auto';
  const step = (now) => {
    const target = glides.get(track);
    place += (target - place) * (1 - Math.exp((last - now) / 90));
    last = now;
    if (Math.abs(target - place) < 0.5) {
      track.scrollLeft = target;
      track.style.scrollBehavior = '';
      glides.delete(track);
      return;
    }
    track.scrollLeft = place;
    requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
}

// Pressing shrinks a button slightly, so a press at its very edge can slip off before
// release, and the browser wouldn't count that as a click. If the pointer is still
// inside where the button was, it counts. Dragging further away still cancels.
let pressed = null;

addEventListener('pointerdown', (event) => {
  const button = event.target.closest?.('button');
  pressed = button && { button, box: button.getBoundingClientRect() };
}, true);

addEventListener('click', (event) => {
  const press = pressed;
  pressed = null;
  if (!press || press.button.contains(event.target) || !event.target.contains(press.button)) return;
  const { left, right, top, bottom } = press.box;
  if (event.clientX >= left && event.clientX <= right && event.clientY >= top && event.clientY <= bottom) press.button.click();
}, true);

// A preview leans toward the pointer, like a sight tracking its target.
let aimed = null;

addEventListener('pointermove', (event) => {
  const target = calm() ? null : event.target.closest?.('.tile, .base');
  if (aimed && aimed !== target) aimed.style.removeProperty('--aim');
  aimed = target;
  if (!target) return;
  const box = target.getBoundingClientRect();
  const lean = (offset, size) => `${((offset / size) - 0.5) * 8}px`;
  target.style.setProperty('--aim', `${lean(event.clientX - box.left, box.width)} ${lean(event.clientY - box.top, box.height)}`);
}, { passive: true });

// The Home logo draws itself: the ring closes, the ticks snap in, the name follows.
function drawLogo(wait = 0) {
  const mark = document.querySelector('.logo-mark');
  if (!mark || calm()) return;
  const [ring, ticks] = mark.querySelector('svg').children;
  ring.animate({ strokeDashoffset: [1, 0], opacity: [0, 1] }, { duration: 480, delay: wait, easing: EASE, fill: 'backwards' });
  ticks.animate({ scale: [1.7, 1], opacity: [0, 1] }, { duration: 640, delay: wait + 90, easing: POP, fill: 'backwards' });
  mark.querySelector('span').animate({ clipPath: ['inset(-10% 100% -10% 0)', 'inset(-10% -4% -10% 0)'] }, {
    duration: 440, delay: wait + 180, easing: EASE, fill: 'backwards',
  });
}

// The opening moment: two sight lines close in on the logo and it locks on.
function playOpening() {
  if (calm()) return;
  const box = (document.querySelector('.logo-mark svg') ?? document.querySelector('.logo')).getBoundingClientRect();
  const across = document.body.appendChild(el('div', { className: 'sight across' }));
  const down = document.body.appendChild(el('div', { className: 'sight down' }));
  const close = { duration: 620, easing: 'cubic-bezier(.16,.84,.3,1)', fill: 'forwards' };
  across.animate({ translate: ['0 0', `0 ${box.top + box.height / 2 - 1}px`] }, close);
  down.animate({ translate: ['0 0', `${box.left + box.width / 2 - 1}px 0`] }, close);
  for (const line of [across, down]) {
    line.animate({ opacity: [1, 0] }, { duration: 300, delay: 620, fill: 'forwards' }).onfinish = () => line.remove();
  }
  [...document.querySelectorAll('.welcome span')].forEach((word, index) => {
    word.animate({ opacity: [0, 1], translate: ['0 16px', '0 0'] }, {
      duration: 640, delay: 140 + index * 90, easing: SPRING, fill: 'backwards',
    });
  });
  drawLogo(520);
  arrive(260);
}

// Icon buttons say what they do straight away, without the browser's long wait.
const tipBox = document.body.appendChild(Object.assign(document.createElement('div'), { className: 'tip' }));
const TIP_TARGETS = '.tile-button, .chevron, .nudge, .home, .window-button';

function showTip(target) {
  if (!target || target.disabled) {
    tipBox.classList.remove('shown');
    return;
  }
  tipBox.textContent = target.getAttribute('aria-label');
  const box = target.getBoundingClientRect();
  const half = tipBox.offsetWidth / 2;
  tipBox.style.left = `${Math.min(Math.max(box.left + box.width / 2, half + 8), innerWidth - half - 8)}px`;
  tipBox.style.top = `${box.top > 40 ? box.top - tipBox.offsetHeight - 8 : box.bottom + 8}px`;
  tipBox.classList.add('shown');
}

addEventListener('pointerover', (event) => showTip(event.target.closest?.(TIP_TARGETS)));
addEventListener('focusin', (event) => showTip(event.target.matches(':focus-visible') ? event.target.closest(TIP_TARGETS) : null));
addEventListener('pointerdown', () => showTip(null));
addEventListener('scroll', () => showTip(null), true);
addEventListener('hashchange', () => showTip(null));

// Asks before going ahead, with an optional box to fill in. Returns true if you do.
// A second choice can be offered with `also`, which returns the word 'also'.
function askFirst(title, detail, action, { field = null, canCancel = true, also = null } = {}) {
  return new Promise((resolve) => {
    const leave = async (answer) => {
      if (!calm()) {
        await dialog.animate({ opacity: [1, 0], scale: [1, 0.96] }, { duration: 140, easing: 'ease-in', fill: 'forwards' }).finished;
      }
      dialog.close();
      dialog.remove();
      resolve(answer);
    };
    const dialog = el('dialog', {
      className: 'ask card',
      oncancel: (event) => { event.preventDefault(); leave(false); },
      onclick: (event) => {
        // Only a click on the dimmed page cancels, not one on the box's own padding.
        const { left, right, top, bottom } = dialog.getBoundingClientRect();
        const inside = event.clientX >= left && event.clientX <= right && event.clientY >= top && event.clientY <= bottom;
        if (event.target === dialog && !inside) leave(false);
      },
    },
      el('h2', {}, title),
      el('p', {}, detail),
      field,
      el('div', { className: 'actions' },
        canCancel ? el('button', { className: 'button', type: 'button', onclick: () => leave(false) }, 'Cancel') : null,
        also ? el('button', { className: 'button', type: 'button', onclick: () => leave('also') }, also) : null,
        el('button', { className: 'button primary', type: 'button', onclick: () => leave(true) }, action)));
    document.body.append(dialog);
    dialog.showModal();
  });
}
