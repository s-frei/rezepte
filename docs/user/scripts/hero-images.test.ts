import { expect, test } from 'bun:test';
import { corners, matrix3d } from './hero-images';

type Point = [number, number];

// Applies the matrix3d the way CSS does to a point of the screenshot.
function apply(m: string, [x, y]: Point): Point {
  const v = m.slice(9, -1).split(',').map(Number);
  const w = v[3] * x + v[7] * y + v[15];
  return [(v[0] * x + v[4] * y + v[12]) / w, (v[1] * x + v[5] * y + v[13]) / w];
}

test('matrix3d maps the screenshot corners onto the quad', () => {
  const quad: [Point, Point, Point, Point] = [
    [100, 50],
    [420, 70],
    [400, 640],
    [90, 600],
  ];
  const m = matrix3d(360, 780, quad);
  const src: Point[] = [
    [0, 0],
    [360, 0],
    [360, 780],
    [0, 780],
  ];
  src.forEach((p, i) => {
    const [x, y] = apply(m, p);
    expect(x).toBeCloseTo(quad[i][0], 3);
    expect(y).toBeCloseTo(quad[i][1], 3);
  });
});

test('corners finds a rounded, occluded screen by its straight edges', () => {
  // A 200x400 screen at (50,30) with 30px rounded corners, a notch in the top
  // edge and a spoon across the right edge.
  const green = (x: number, y: number) => {
    const [l, t, r, b, rad] = [50, 30, 249, 429, 30];
    if (x < l || x > r || y < t || y > b) return false;
    const cx = Math.min(Math.max(x, l + rad), r - rad);
    const cy = Math.min(Math.max(y, t + rad), b - rad);
    if ((x - cx) ** 2 + (y - cy) ** 2 > rad ** 2) return false;
    if (y < t + 12 && x > 120 && x < 180) return false; // notch
    if (y > 200 && y < 230 && x > 220) return false; // spoon
    return true;
  };
  const found = corners(green, 300, 460);
  const expected: Point[] = [
    [50, 30],
    [249, 30],
    [249, 429],
    [50, 429],
  ];
  found.forEach((p, i) => {
    expect(Math.abs(p[0] - expected[i][0])).toBeLessThan(1.5);
    expect(Math.abs(p[1] - expected[i][1])).toBeLessThan(1.5);
  });
});
