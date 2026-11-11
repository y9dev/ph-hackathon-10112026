// In-page overlay for the backup demo video: an animated cursor, click ripples, a spotlight ring,
// explanation captions and full-screen scene cards. It is injected with page.addInitScript, keeps its
// state in sessionStorage and therefore survives full page navigations. Everything is pointer-events:none.
export const OVERLAY_JS = String.raw`
(() => {
  if (window.__demo) return;
  const store = {
    get(k) { try { return sessionStorage.getItem(k); } catch { return null; } },
    set(k, v) { try { if (v === null) sessionStorage.removeItem(k); else sessionStorage.setItem(k, v); } catch {} },
  };
  const css = [
    '#__demo{position:fixed;inset:0;z-index:2147483000;pointer-events:none;font-family:"Segoe UI",system-ui,sans-serif}',
    '#__demo *{box-sizing:border-box}',
    '#__demo .spot{position:fixed;border-radius:14px;border:3px solid #ffd24a;box-shadow:0 0 0 9999px rgba(8,12,24,.62),0 0 26px 4px rgba(255,210,74,.55);opacity:0;transition:opacity .45s ease,left .5s cubic-bezier(.3,.7,.2,1),top .5s cubic-bezier(.3,.7,.2,1),width .5s cubic-bezier(.3,.7,.2,1),height .5s cubic-bezier(.3,.7,.2,1)}',
    '#__demo .cap{position:fixed;left:50%;max-width:980px;min-width:420px;transform:translate(-50%,16px);opacity:0;transition:opacity .4s ease,transform .4s ease;background:rgba(10,16,32,.94);color:#fff;border-radius:16px;padding:14px 24px 15px;font-size:23px;line-height:1.35;font-weight:500;text-align:center;box-shadow:0 10px 36px rgba(0,0,0,.45);border:1px solid rgba(255,255,255,.14)}',
    '#__demo .cap.top{top:18px}#__demo .cap.bottom{bottom:22px}',
    '#__demo .cap.on{opacity:1;transform:translate(-50%,0)}',
    '#__demo .cap small{display:block;font-size:13px;letter-spacing:.14em;text-transform:uppercase;color:#ffd24a;margin-bottom:4px;font-weight:700}',
    '#__demo .cur{position:fixed;left:0;top:0;width:34px;height:34px;filter:drop-shadow(0 3px 5px rgba(0,0,0,.45));will-change:transform}',
    '#__demo .cur svg{display:block;transform-origin:3px 2px;transition:transform .12s ease}',
    '#__demo .cur.down svg{transform:scale(.82)}',
    '#__demo .rip{position:fixed;width:46px;height:46px;margin:-23px 0 0 -23px;border-radius:50%;border:3px solid #ffd24a;background:rgba(255,210,74,.28);animation:__rip .6s ease-out forwards}',
    '@keyframes __rip{from{transform:scale(.2);opacity:.9}to{transform:scale(1.9);opacity:0}}',
    '#__demo .card{position:fixed;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:14px;background:radial-gradient(ellipse at 30% 20%,#23407a 0%,#0f1b36 55%,#0a1124 100%);color:#fff;text-align:center;padding:0 90px;opacity:0;transition:opacity .5s ease}',
    '#__demo .card.on{opacity:1}',
    '#__demo .card .k{font-size:18px;letter-spacing:.22em;text-transform:uppercase;color:#ffd24a;font-weight:700}',
    '#__demo .card .t{font-size:54px;line-height:1.12;font-weight:700;max-width:1020px}',
    '#__demo .card .s{font-size:25px;line-height:1.4;color:#c4d0ee;max-width:900px}',
  ].join('');

  const state = { x: 0, y: 0, cap: null, card: null };
  try { Object.assign(state, JSON.parse(store.get('demo.state') || '{}')); } catch {}
  const save = () => store.set('demo.state', JSON.stringify(state));

  let root, spot, cap, cur, card, booted = false;
  const el = (cls, parent) => { const d = document.createElement('div'); d.className = cls; parent.appendChild(d); return d; };
  const esc = (s) => String(s).replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' }[c]));

  function placeCursor() { cur.style.transform = 'translate(' + state.x + 'px,' + state.y + 'px)'; }
  function renderCap(animate) {
    if (!state.cap) { cap.classList.remove('on'); return; }
    cap.className = 'cap ' + (state.cap.pos || 'bottom');
    cap.innerHTML = (state.cap.kicker ? '<small>' + esc(state.cap.kicker) + '</small>' : '') + esc(state.cap.text);
    if (animate) requestAnimationFrame(() => cap.classList.add('on')); else cap.classList.add('on');
  }
  function renderCard(animate) {
    if (!state.card) { card.classList.remove('on'); card.innerHTML = ''; return; }
    card.innerHTML = '<div class="k">' + esc(state.card.kicker || '') + '</div><div class="t">' + esc(state.card.title || '') +
      '</div><div class="s">' + esc(state.card.sub || '') + '</div>';
    if (animate) requestAnimationFrame(() => requestAnimationFrame(() => card.classList.add('on'))); else card.classList.add('on');
  }
  function boot() {
    if (booted) return; booted = true;
    const st = document.createElement('style'); st.textContent = css; document.documentElement.appendChild(st);
    root = document.createElement('div'); root.id = '__demo'; document.documentElement.appendChild(root);
    spot = el('spot', root); cap = el('cap', root);
    cur = el('cur', root);
    cur.innerHTML = '<svg viewBox="0 0 24 24" width="34" height="34"><path d="M3 2l7.5 19 2.6-7.6L21 10.8z" fill="#fff" stroke="#111" stroke-width="1.6" stroke-linejoin="round"/></svg>';
    card = el('card', root);
    if (!state.x && !state.y) { state.x = innerWidth * 0.62; state.y = innerHeight * 0.78; }
    placeCursor(); renderCap(false); renderCard(false);
  }
  if (document.documentElement) boot(); else document.addEventListener('DOMContentLoaded', boot);

  const ease = (t) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2);
  window.__demo = {
    pos: () => ({ x: state.x, y: state.y }),
    moveTo(x, y, ms) {
      return new Promise((resolve) => {
        const x0 = state.x, y0 = state.y, dx = x - x0, dy = y - y0, dist = Math.hypot(dx, dy) || 1;
        const bend = Math.min(90, dist * 0.14) * (dx >= 0 ? -1 : 1);
        const cx = (x0 + x) / 2 + (-dy / dist) * bend, cy = (y0 + y) / 2 + (dx / dist) * bend;
        const t0 = performance.now();
        const step = (now) => {
          const p = Math.min(1, (now - t0) / ms), e = ease(p), u = 1 - e;
          state.x = u * u * x0 + 2 * u * e * cx + e * e * x; state.y = u * u * y0 + 2 * u * e * cy + e * e * y;
          placeCursor();
          if (p < 1) requestAnimationFrame(step); else { save(); resolve(); }
        };
        requestAnimationFrame(step);
      });
    },
    press() {
      cur.classList.add('down');
      const r = el('rip', root); r.style.left = state.x + 'px'; r.style.top = state.y + 'px';
      setTimeout(() => cur.classList.remove('down'), 140); setTimeout(() => r.remove(), 700);
    },
    spot(rect, pad) {
      if (!rect) { spot.style.opacity = '0'; return; }
      const p = pad == null ? 10 : pad;
      const hidden = spot.style.opacity !== '1';
      if (hidden) spot.style.transition = 'none';
      spot.style.left = rect.x - p + 'px'; spot.style.top = rect.y - p + 'px';
      spot.style.width = rect.width + 2 * p + 'px'; spot.style.height = rect.height + 2 * p + 'px';
      if (hidden) { void spot.offsetWidth; spot.style.transition = ''; }
      spot.style.opacity = '1';
    },
    caption(text, pos, kicker) {
      state.cap = text ? { text, pos: pos || 'bottom', kicker: kicker || '' } : null; save(); renderCap(true);
    },
    card(kicker, title, sub) { state.card = { kicker, title, sub }; save(); renderCard(true); },
    cardOff() { state.card = null; save(); card.classList.remove('on'); },
  };
})();
`;
