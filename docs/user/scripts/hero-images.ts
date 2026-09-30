// Paints the wide mascot scenes for the README and the docs: each scene from
// assets/brand/scenes, the current docs screenshot on its green screen and,
// where the picture heads a README or a home page, the wordmark. Runs after
// `mise run //docs/user:screenshots`, and on its own as
// `mise run //docs/user:hero-images`.
//
// A generated screen is never quite square to the frame, so the screenshot is
// not pasted but mapped onto the screen's four corners with a CSS matrix3d and
// clipped to the green pixels in Chromium, which keeps whatever stands in front
// of the screen (a spoon, a hand) in front of it.
import { chromium } from "@playwright/test";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import sharp from "sharp";

type Point = [number, number];
type Scene = {
  /** File in assets/brand/scenes, without extension. */
  name: string;
  /** Docs screenshot on the green screen, without `-light.png`/`-dark.png`. */
  screen?: string;
  /** Where the painting ends; the picture is cut there. */
  cropRight?: number;
  /**
   * The wordmark's center and width, in scene pixels. `glow` lays a soft
   * patch of the wall's own color behind it where the wall is patterned.
   */
  wordmark?: { x: number; y: number; width: number; glow?: boolean };
};

// kitchen: the README header. guests: the docs home. laptop and phone sit on
// pages that already carry a title, so they go without the wordmark and are
// cut where the painting ends.
export const SCENES: Scene[] = [
  { name: "kitchen", wordmark: { x: 2660, y: 610, width: 760 } },
  {
    name: "guests",
    wordmark: { x: 2560, y: 560, width: 820, glow: true },
  },
  { name: "laptop", screen: "overview-desktop", cropRight: 2430 },
  { name: "phone", screen: "cook-mode-mobile", cropRight: 2096 },
];

const OUTPUT_WIDTH = 1600;

/** A pixel counts as screen when it is clearly green. */
export function isGreen(r: number, g: number, b: number): boolean {
  return g > 150 && g - Math.max(r, b) > 80;
}

// A line v = a + b·t through the points of one screen edge. Whatever covers
// the edge (a spoon, the camera notch) only ever pulls the outermost green
// pixel inward, so each pass drops the points that lie inward of the last fit.
// `outward` is +1 when larger values lie outside the screen, -1 otherwise.
function fitLine(points: Point[], outward: 1 | -1): [number, number] {
  const fit = (ps: Point[]): [number, number] => {
    const n = ps.length;
    const mt = ps.reduce((s, p) => s + p[0], 0) / n;
    const mv = ps.reduce((s, p) => s + p[1], 0) / n;
    const cov = ps.reduce((s, [t, v]) => s + (t - mt) * (v - mv), 0);
    const varT = ps.reduce((s, [t]) => s + (t - mt) ** 2, 0) || 1;
    const b = cov / varT;
    return [mv - b * mt, b];
  };
  let line = fit(points);
  for (let pass = 0; pass < 4; pass++) {
    const [a, b] = line;
    const kept = points.filter(([t, v]) => outward * (v - (a + b * t)) >= -2);
    if (kept.length < 10) break;
    line = fit(kept);
  }
  return line;
}

/**
 * The screen's corners. Phone screens have rounded corners, so the corners
 * are where the four straight edges meet, each edge fitted through the
 * outermost green pixels of the rows or columns in its middle 70 %.
 */
export function corners(
  green: (x: number, y: number) => boolean,
  width: number,
  height: number,
): [Point, Point, Point, Point] {
  let x0 = width,
    x1 = 0,
    y0 = height,
    y1 = 0;
  for (let y = 0; y < height; y++)
    for (let x = 0; x < width; x++)
      if (green(x, y)) {
        x0 = Math.min(x0, x);
        x1 = Math.max(x1, x);
        y0 = Math.min(y0, y);
        y1 = Math.max(y1, y);
      }
  const inner = (lo: number, hi: number) => [
    Math.round(lo + (hi - lo) * 0.15),
    Math.round(hi - (hi - lo) * 0.15),
  ];
  const left: Point[] = [],
    right: Point[] = [],
    top: Point[] = [],
    bottom: Point[] = [];
  const [ry0, ry1] = inner(y0, y1);
  for (let y = ry0; y <= ry1; y++) {
    let l = -1,
      r = -1;
    for (let x = x0; x <= x1; x++)
      if (green(x, y)) {
        if (l < 0) l = x;
        r = x;
      }
    if (l >= 0) {
      left.push([y, l]);
      right.push([y, r]);
    }
  }
  const [cx0, cx1] = inner(x0, x1);
  for (let x = cx0; x <= cx1; x++) {
    let t = -1,
      b = -1;
    for (let y = y0; y <= y1; y++)
      if (green(x, y)) {
        if (t < 0) t = y;
        b = y;
      }
    if (t >= 0) {
      top.push([x, t]);
      bottom.push([x, b]);
    }
  }
  const [la, lb] = fitLine(left, -1),
    [ra, rb] = fitLine(right, 1); // x = a + b·y
  const [ta, tb] = fitLine(top, -1),
    [ba, bb] = fitLine(bottom, 1); // y = a + b·x
  const meet = (xa: number, xb: number, ya: number, yb: number): Point => {
    const y = (ya + yb * xa) / (1 - yb * xb);
    return [xa + xb * y, y];
  };
  return [
    meet(la, lb, ta, tb),
    meet(ra, rb, ta, tb),
    meet(ra, rb, ba, bb),
    meet(la, lb, ba, bb),
  ];
}

// Solves the homography that takes the rectangle (0,0)-(w,h) to the quad,
// returned as a CSS matrix3d (column-major) for transform-origin 0 0.
export function matrix3d(
  w: number,
  h: number,
  quad: [Point, Point, Point, Point],
): string {
  const src: Point[] = [
    [0, 0],
    [w, 0],
    [w, h],
    [0, h],
  ];
  const rows: number[][] = [];
  quad.forEach(([u, v], i) => {
    const [x, y] = src[i];
    rows.push([x, y, 1, 0, 0, 0, -u * x, -u * y, u]);
    rows.push([0, 0, 0, x, y, 1, -v * x, -v * y, v]);
  });
  for (let c = 0; c < 8; c++) {
    const pivot = rows.reduce(
      (best, _, r) =>
        r >= c && Math.abs(rows[r][c]) > Math.abs(rows[best][c]) ? r : best,
      c,
    );
    [rows[c], rows[pivot]] = [rows[pivot], rows[c]];
    for (let r = 0; r < 8; r++) {
      if (r === c) continue;
      const k = rows[r][c] / rows[c][c];
      for (let j = c; j < 9; j++) rows[r][j] -= k * rows[c][j];
    }
  }
  const [a, b, cc, d, e, f, g, hh] = rows.map((row, i) => row[8] / row[i]);
  return `matrix3d(${[a, d, 0, g, b, e, 0, hh, 0, 0, 1, 0, cc, f, 0, 1].join(",")})`;
}

// Pushes each corner a few pixels away from the center, so the screenshot
// also covers the anti-aliased rim the mask lets through.
function grow(
  quad: [Point, Point, Point, Point],
  by: number,
): [Point, Point, Point, Point] {
  const cx = quad.reduce((s, p) => s + p[0], 0) / 4;
  const cy = quad.reduce((s, p) => s + p[1], 0) / 4;
  return quad.map(([x, y]) => {
    const len = Math.hypot(x - cx, y - cy);
    return [x + ((x - cx) / len) * by, y + ((y - cy) / len) * by];
  }) as [Point, Point, Point, Point];
}

async function screenMask(file: string) {
  const { data, info } = await sharp(file)
    .removeAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true });
  const alpha = Buffer.alloc(info.width * info.height);
  for (let i = 0; i < alpha.length; i++) {
    if (isGreen(data[i * 3], data[i * 3 + 1], data[i * 3 + 2])) alpha[i] = 255;
  }
  // Widen the mask by a pixel or two, over the green fringe of the edge.
  const widened = await sharp(alpha, {
    raw: { width: info.width, height: info.height, channels: 1 },
  })
    .blur(1.5)
    .threshold(40)
    .toColourspace("b-w")
    .raw()
    .toBuffer();
  // CSS masks by alpha, so the screen goes into the alpha channel of a white
  // picture; an opaque grayscale mask would clip nothing.
  const mask = await sharp({
    create: {
      width: info.width,
      height: info.height,
      channels: 3,
      background: "#ffffff",
    },
  })
    .joinChannel(widened, {
      raw: { width: info.width, height: info.height, channels: 1 },
    })
    .png()
    .toBuffer();
  const green = (x: number, y: number) => alpha[y * info.width + x] === 255;
  return { mask, quad: corners(green, info.width, info.height) };
}

/** The wall behind the wordmark without its pattern (mean plus one deviation), for its glow. */
async function wallColor(
  file: string,
  w: { x: number; y: number; width: number },
): Promise<string> {
  const height = Math.round(w.width / 3);
  const { channels } = await sharp(file)
    .extract({
      left: Math.round(w.x - w.width / 2),
      top: Math.round(w.y - height / 2),
      width: w.width,
      height,
    })
    .stats();
  const [r, g, b] = channels.map((c) =>
    Math.min(255, Math.round(c.mean + c.stdev)),
  );
  return `rgb(${r}, ${g}, ${b})`;
}

async function main() {
  const root = new URL("../../../", import.meta.url);
  const scenes = new URL("assets/brand/scenes/", root);
  const shots = new URL("../public/screenshots/", import.meta.url);
  const target = new URL("../public/heroes/", import.meta.url);
  const wordmark = new URL("assets/brand/lockups/rezepte-wordmark.svg", root)
    .href;
  await mkdir(target, { recursive: true });

  // A file:// page, not setContent: about:blank may not load file:// images.
  const work = await mkdtemp(join(tmpdir(), "rezepte-heroes-"));
  const browser = await chromium.launch();
  try {
    for (const scene of SCENES) {
      const file = new URL(`${scene.name}.jpg`, scenes);
      const { width, height } = await sharp(file.pathname).metadata();
      const screen = scene.screen ? await screenMask(file.pathname) : undefined;
      const page = await browser.newPage({
        viewport: { width: width!, height: height! },
      });

      for (const scheme of scene.screen
        ? (["light", "dark"] as const)
        : (["light"] as const)) {
        let shotLayer = "";
        if (screen && scene.screen) {
          const shot = new URL(`${scene.screen}-${scheme}.png`, shots);
          const meta = await sharp(shot.pathname).metadata();
          const transform = matrix3d(
            meta.width!,
            meta.height!,
            grow(screen.quad, 3),
          );
          const mask = `data:image/png;base64,${screen.mask.toString("base64")}`;
          shotLayer = `<div style="position:absolute;inset:0;-webkit-mask-image:url(${mask});mask-image:url(${mask});mask-size:100% 100%">
						<img src="${shot.href}" style="position:absolute;left:0;top:0;width:${meta.width}px;height:${meta.height}px;transform-origin:0 0;transform:${transform}"></div>`;
        }
        const w = scene.wordmark;
        const glow = w?.glow
          ? `<div style="position:absolute;left:${w.x - w.width * 0.8}px;top:${w.y - w.width * 0.45}px;width:${w.width * 1.6}px;height:${w.width * 0.9}px;background:radial-gradient(closest-side, ${await wallColor(file.pathname, w)} 55%, transparent)"></div>`
          : "";
        const title = w
          ? `<img src="${wordmark}" style="position:absolute;left:${w.x - w.width / 2}px;top:${w.y}px;width:${w.width}px;transform:translateY(-50%)">`
          : "";
        const html = `<body style="margin:0;width:${width}px;height:${height}px;position:relative;overflow:hidden">
					<img src="${file.href}" style="position:absolute;inset:0">${shotLayer}${glow}${title}</body>`;
        const htmlFile = join(work, "scene.html");
        await writeFile(htmlFile, html);
        await page.goto(`file://${htmlFile}`, { waitUntil: "load" });
        const png = await page.screenshot({
          type: "png",
          clip: {
            x: 0,
            y: 0,
            width: scene.cropRight ?? width!,
            height: height!,
          },
        });
        const name = scene.screen
          ? `${scene.name}-${scheme}.jpg`
          : `${scene.name}.jpg`;
        await writeFile(
          new URL(name, target),
          await sharp(png)
            .resize({ width: OUTPUT_WIDTH })
            .jpeg({ quality: 86, mozjpeg: true })
            .toBuffer(),
        );
      }
      await page.close();
    }
  } finally {
    await browser.close();
    await rm(work, { recursive: true, force: true });
  }
}

if (process.argv[1] === new URL(import.meta.url).pathname) await main();
