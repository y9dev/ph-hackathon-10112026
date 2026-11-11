// Renders pitch/demo-backup.mp4 from the frames and sound timeline written by e2e/demo-backup.spec.ts.
// Needs an ffmpeg with libx264 and aac. Set FFMPEG=path if it is not on PATH.
//   node scripts/make-demo-video.mjs [framesDir] [outFile]
import { spawnSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const dir = resolve(process.argv[2] ?? process.env.DEMO_FRAMES_DIR ?? join(tmpdir(), 'modus-demo-frames'));
const out = resolve(process.argv[3] ?? '../pitch/demo-backup.mp4');
const ffmpeg = process.env.FFMPEG ?? 'ffmpeg';
const { frames, events, duration } = JSON.parse(readFileSync(join(dir, 'timeline.json'), 'utf8'));

// ---- video list (concat demuxer with per-frame durations) -------------------------------------
const lines = ['ffconcat version 1.0'];
frames.forEach((f, i) => {
  const next = i + 1 < frames.length ? frames[i + 1].t : duration;
  lines.push(`file '${f.file}'`, `duration ${Math.max(0.001, next - f.t).toFixed(4)}`);
});
lines.push(`file '${frames[frames.length - 1].file}'`);
writeFileSync(join(dir, 'frames.txt'), lines.join('\n'));

// ---- soundtrack ----------------------------------------------------------------------------------
const SR = 44100;
const total = Math.ceil((duration + 0.5) * SR);
const L = new Float32Array(total);
const R = new Float32Array(total);
let seed = 7;
const rnd = () => ((seed = (seed * 1664525 + 1013904223) >>> 0) / 4294967296);
const hz = (midi) => 440 * Math.pow(2, (midi - 69) / 12);
function add(buf, start, len, fn, gain) {
  const s0 = Math.floor(start * SR);
  const n = Math.min(Math.floor(len * SR), total - s0);
  for (let i = 0; i < n; i++) buf[s0 + i] += fn(i / SR) * gain;
}

// Pad + bass + soft plucks over Cmaj7 - Am7 - Fmaj7 - G6, 72 bpm, one chord per two bars.
const chords = [
  [48, 52, 55, 59],
  [45, 52, 55, 60],
  [41, 48, 52, 57],
  [43, 50, 52, 59],
];
const beat = 60 / 72;
const chordLen = beat * 8;
for (let c = 0, t = 0; t < duration + chordLen; c++, t += chordLen) {
  const notes = chords[c % chords.length];
  notes.forEach((m, idx) => {
    for (const [buf, det] of [[L, 1.0015], [R, 0.9985]]) {
      const f = hz(m) * det;
      add(buf, t, chordLen + 2.2, (x) => {
        const a = Math.min(1, x / 1.6), r = Math.min(1, Math.max(0, (chordLen + 2.2 - x) / 2.2));
        const tr = 1 + 0.06 * Math.sin(2 * Math.PI * 0.18 * x + idx);
        return (Math.sin(2 * Math.PI * f * x) + 0.28 * Math.sin(4 * Math.PI * f * x) + 0.1 * Math.sin(6 * Math.PI * f * x)) * a * r * tr;
      }, 0.034);
    }
  });
  const root = hz(notes[0] - 12);
  add(L, t, 2.6, (x) => Math.sin(2 * Math.PI * root * x) * Math.exp(-x / 1.3) * Math.min(1, x / 0.03), 0.09);
  add(R, t, 2.6, (x) => Math.sin(2 * Math.PI * root * x) * Math.exp(-x / 1.3) * Math.min(1, x / 0.03), 0.09);
  for (let k = 0; k < 16; k++) {
    if (rnd() < 0.3) continue;
    const m = notes[Math.floor(rnd() * notes.length)] + 24;
    const f = hz(m), st = t + k * beat / 2, pan = rnd();
    const fn = (x) => Math.sin(2 * Math.PI * f * x) * Math.exp(-x / 0.4) * Math.min(1, x / 0.004);
    add(L, st, 1.8, fn, 0.034 * (1 - pan * 0.7));
    add(R, st, 1.8, fn, 0.034 * (0.3 + pan * 0.7));
  }
  for (let k = 1; k < 16; k += 2) {
    const st = t + k * beat / 2;
    const fn = (x) => (rnd() * 2 - 1) * Math.exp(-x / 0.012);
    add(L, st, 0.08, fn, 0.011);
    add(R, st, 0.08, fn, 0.011);
  }
}

// Sound effects driven by the recorded events.
const both = (st, len, fn, g) => { add(L, st, len, fn, g); add(R, st, len, fn, g); };
const tone = (f, tau, att = 0.003) => (x) => Math.sin(2 * Math.PI * f * x) * Math.exp(-x / tau) * Math.min(1, x / att);
const noise = (tau) => (x) => (rnd() * 2 - 1) * Math.exp(-x / tau);
for (const e of events) {
  const t = e.t;
  if (e.kind === 'click') {
    both(t, 0.12, tone(1100, 0.012, 0.0005), 0.16);
    both(t, 0.12, tone(190, 0.03, 0.0005), 0.18);
    both(t, 0.04, noise(0.006), 0.07);
  } else if (e.kind === 'type') {
    for (let i = 0; i < (e.n ?? 0); i++) {
      const st = t + i * (e.d ?? 0.04);
      both(st, 0.05, noise(0.005), 0.045 * (0.6 + rnd() * 0.5));
      both(st, 0.06, tone(700 + rnd() * 500, 0.008, 0.0005), 0.02);
    }
  } else if (e.kind === 'card') {
    [523.25, 659.25, 783.99, 1046.5].forEach((f, i) => both(t + i * 0.09, 1.8, tone(f, 0.55, 0.01), 0.07));
  } else if (e.kind === 'ok') {
    [523.25, 659.25, 783.99, 1046.5].forEach((f, i) => both(t + i * 0.08, 0.9, tone(f, 0.22, 0.004), 0.085));
  } else if (e.kind === 'block') {
    both(t, 0.9, (x) => Math.sin(2 * Math.PI * (90 - 40 * Math.min(1, x / 0.4)) * x) * Math.exp(-x / 0.25) * Math.min(1, x / 0.002), 0.34);
    both(t, 0.25, noise(0.03), 0.06);
  } else if (e.kind === 'spot') {
    both(t, 1.0, tone(1318.5, 0.3, 0.004), 0.05);
  }
}

// Normalise, fade, write 16-bit WAV.
let peak = 0;
for (let i = 0; i < total; i++) peak = Math.max(peak, Math.abs(L[i]), Math.abs(R[i]));
const norm = 0.82 / (peak || 1);
const fadeIn = 1.2 * SR, fadeOut = 2.5 * SR, endAt = Math.floor(duration * SR);
const pcm = Buffer.alloc(44 + total * 4);
pcm.write('RIFF', 0); pcm.writeUInt32LE(36 + total * 4, 4); pcm.write('WAVEfmt ', 8);
pcm.writeUInt32LE(16, 16); pcm.writeUInt16LE(1, 20); pcm.writeUInt16LE(2, 22); pcm.writeUInt32LE(SR, 24);
pcm.writeUInt32LE(SR * 4, 28); pcm.writeUInt16LE(4, 32); pcm.writeUInt16LE(16, 34); pcm.write('data', 36); pcm.writeUInt32LE(total * 4, 40);
for (let i = 0; i < total; i++) {
  const g = norm * Math.min(1, i / fadeIn) * Math.min(1, Math.max(0, (endAt - i) / fadeOut));
  pcm.writeInt16LE(Math.max(-32767, Math.min(32767, Math.round(L[i] * g * 32767))), 44 + i * 4);
  pcm.writeInt16LE(Math.max(-32767, Math.min(32767, Math.round(R[i] * g * 32767))), 46 + i * 4);
}
writeFileSync(join(dir, 'soundtrack.wav'), pcm);

// ---- mux ---------------------------------------------------------------------------------------------
const args = [
  '-y', '-hide_banner', '-loglevel', 'error',
  '-f', 'concat', '-safe', '0', '-i', join(dir, 'frames.txt'),
  '-i', join(dir, 'soundtrack.wav'),
  '-vf', `fps=30,eq=contrast=0.95:brightness=0.04:saturation=0.96:gamma=1.1,unsharp=5:5:-0.45:5:5:0,vignette=angle=PI/6,fade=t=in:st=0:d=0.5,fade=t=out:st=${Math.max(0, duration - 0.9).toFixed(2)}:d=0.9,format=yuv420p`,
  '-c:v', 'libx264', '-preset', 'slow', '-crf', '20', '-movflags', '+faststart',
  '-c:a', 'aac', '-b:a', '160k', '-t', duration.toFixed(2), out,
];
const r = spawnSync(ffmpeg, args, { stdio: 'inherit' });
if (r.status !== 0) {
  console.error('ffmpeg failed; set FFMPEG to a build with libx264 and aac');
  process.exit(r.status ?? 1);
}
console.log(`wrote ${out} (${duration.toFixed(1)} s)`);
