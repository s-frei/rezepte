// The logo's six colors. They mirror the --color-logo-* tokens in
// src/app.css; standalone image files cannot read CSS, so the values are
// written into them literally and this is the one list every generator and
// test checks against.
export const LIGHT = { ink: '#7c3d0a', sprout: '#7fb069', ground: '#f4ede2' } as const;
export const DARK = { ink: '#f0c391', sprout: '#aed3a8', ground: '#1c1612' } as const;
export const ALLOWED = new Set<string>([...Object.values(LIGHT), ...Object.values(DARK)]);
