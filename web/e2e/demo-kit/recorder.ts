/* eslint-disable @typescript-eslint/no-explicit-any -- window.__demo is the injected overlay API */
import type { CDPSession, Locator, Page } from '@playwright/test';
import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { OVERLAY_JS } from './overlay';

export interface DemoEvent {
  t: number;
  kind: 'click' | 'type' | 'card' | 'ok' | 'block' | 'spot';
  n?: number; // type: number of characters
  d?: number; // type: delay per character, seconds
}

/**
 * Records a page through CDP screencast (JPEG frames with real timestamps) instead of Playwright's
 * low-bitrate VP8 video, and keeps a timeline of sound events. Recording can be paused so that logins and
 * other preparation stay out of the video. web/scripts/make-demo-video.mjs turns the output into an mp4.
 */
export class DemoRecorder {
  private cdp!: CDPSession;
  private writes: Promise<void>[] = [];
  private rawFrames: { file: string; wall: number }[] = [];
  private rawEvents: { wall: number; kind: DemoEvent['kind']; n?: number; d?: number }[] = [];
  private ranges: { from: number; to: number }[] = [];
  private pausedAt: number | null = null;
  private seq = 0;

  constructor(private page: Page, private dir: string) {}

  async start() {
    rmSync(this.dir, { recursive: true, force: true });
    mkdirSync(this.dir, { recursive: true });
    this.cdp = await this.page.context().newCDPSession(this.page);
    this.cdp.on('Page.screencastFrame', (e) => {
      void this.cdp.send('Page.screencastFrameAck', { sessionId: e.sessionId }).catch(() => undefined);
      if (this.pausedAt !== null) return;
      const file = `f${String(++this.seq).padStart(6, '0')}.jpg`;
      this.rawFrames.push({ file, wall: e.metadata.timestamp ?? Date.now() / 1000 });
      this.writes.push(writeFile(join(this.dir, file), Buffer.from(e.data, 'base64')));
    });
    await this.cdp.send('Page.startScreencast', { format: 'jpeg', quality: 90, maxWidth: 1920, maxHeight: 1080, everyNthFrame: 1 });
  }

  pause() {
    if (this.pausedAt === null) this.pausedAt = Date.now() / 1000;
  }
  resume() {
    if (this.pausedAt !== null) this.ranges.push({ from: this.pausedAt, to: Date.now() / 1000 });
    this.pausedAt = null;
  }
  mark(kind: DemoEvent['kind'], extra: { n?: number; d?: number } = {}) {
    if (this.pausedAt === null) this.rawEvents.push({ wall: Date.now() / 1000, kind, ...extra });
  }

  /** Converts a raw wall-clock time to video time (paused stretches removed). Returns null inside a pause. */
  private adjust(wall: number): number | null {
    let cut = 0;
    for (const r of this.ranges) {
      if (wall >= r.from && wall <= r.to) return null;
      if (r.to < wall) cut += r.to - r.from;
    }
    return wall - cut;
  }

  async stop(holdSeconds = 1.5) {
    await this.cdp.send('Page.stopScreencast').catch(() => undefined);
    await Promise.all(this.writes);
    const frames = this.rawFrames
      .map((f) => ({ file: f.file, at: this.adjust(f.wall) }))
      .filter((f): f is { file: string; at: number } => f.at !== null);
    const base = frames[0]?.at ?? 0;
    const timeline = frames.map((f) => ({ file: f.file, t: Math.max(0, f.at - base) }));
    const events = this.rawEvents
      .map((e) => ({ ...e, at: this.adjust(e.wall) }))
      .filter((e) => e.at !== null)
      .map((e) => ({ t: Math.max(0, (e.at as number) - base), kind: e.kind, n: e.n, d: e.d }));
    const last = timeline[timeline.length - 1]?.t ?? 0;
    const duration = last + holdSeconds;
    writeFileSync(join(this.dir, 'timeline.json'), JSON.stringify({ frames: timeline, events, duration }, null, 1));
    return { frames: timeline.length, duration, events: events.length };
  }
}

const sleep = (page: Page, ms: number) => page.waitForTimeout(ms);

/** High-level, human-paced actions on top of the overlay. */
export class Director {
  constructor(public page: Page, public rec: DemoRecorder) {}

  async install() {
    await this.page.addInitScript(OVERLAY_JS);
  }

  /** Waits so the viewer can read: roughly 30 ms per character, at least 1.5 s. */
  readingTime(text: string) {
    return Math.max(1100, text.length * 22);
  }

  async sceneCard(kicker: string, title: string, sub: string, holdMs = 800) {
    await this.page.evaluate(([k, t, s]) => {
      (window as any).__demo.caption('');
      (window as any).__demo.card(k, t, s);
    }, [kicker, title, sub]);
    this.rec.mark('card');
    await sleep(this.page, holdMs);
  }
  async cardOff() {
    await this.page.evaluate(() => (window as any).__demo.cardOff());
    await sleep(this.page, 450);
  }

  async caption(text: string, pos: 'top' | 'bottom' = 'bottom', kicker = '') {
    await this.page.evaluate(([t, p, k]) => (window as any).__demo.caption(t, p, k), [text, pos, kicker]);
  }
  async captionOff() {
    await this.page.evaluate(() => (window as any).__demo.caption(''));
    await sleep(this.page, 450);
  }

  private async reveal(loc: Locator) {
    const box = await loc.boundingBox();
    const h = this.page.viewportSize()!.height;
    if (!box || box.y < 90 || box.y + box.height > h - 150) {
      await loc.evaluate((e) => (e as HTMLElement).scrollIntoView({ block: 'center', behavior: 'smooth' }));
      await sleep(this.page, 900);
    }
  }

  private async center(loc: Locator) {
    await this.reveal(loc);
    const box = (await loc.boundingBox())!;
    return { x: box.x + box.width / 2, y: box.y + Math.min(box.height / 2, 30), box };
  }

  /** Flies the cursor to an element, slowly. */
  async moveTo(loc: Locator, ms = 600) {
    const { x, y } = await this.center(loc);
    await this.page.evaluate(([px, py, d]) => (window as any).__demo.moveTo(px, py, d), [x, y, ms]);
    await sleep(this.page, 120);
  }

  async click(loc: Locator, ms = 600) {
    await this.moveTo(loc, ms);
    await this.page.evaluate(() => (window as any).__demo.press());
    this.rec.mark('click');
    await sleep(this.page, 160);
    await loc.click();
    await sleep(this.page, 250);
  }

  /** Clicks an input and types into it character by character. */
  async type(loc: Locator, text: string, delay = 20) {
    await this.click(loc);
    this.rec.mark('type', { n: text.length, d: delay / 1000 });
    await loc.pressSequentially(text, { delay });
    await sleep(this.page, 250);
  }

  /** The "slow-motion" moment: dims everything but the target, shows the caption and holds. */
  async explain(loc: Locator | null, text: string, opts: { pos?: 'top' | 'bottom'; kicker?: string; extraMs?: number; pad?: number } = {}) {
    if (loc) {
      await this.reveal(loc);
      const box = (await loc.boundingBox())!;
      await this.page.evaluate(([b, pad]) => (window as any).__demo.spot(b, pad), [box, opts.pad ?? 10] as const);
      this.rec.mark('spot');
    }
    const pos = opts.pos ?? (loc && (await loc.boundingBox())!.y > this.page.viewportSize()!.height * 0.5 ? 'top' : 'bottom');
    await this.caption(text, pos, opts.kicker ?? '');
    await sleep(this.page, this.readingTime(text) + (opts.extraMs ?? 0));
    if (loc) {
      await this.page.evaluate(() => (window as any).__demo.spot(null));
      await sleep(this.page, 300);
    }
  }

  pause(ms: number) {
    return sleep(this.page, ms);
  }
}
