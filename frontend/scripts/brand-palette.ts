// The logo's colors. They live in image files, not in app.css: the lockups
// carry their own ground, so they look the same in both themes and no UI
// token reaches them. This is the one list every generator and test checks.

/** The mascot's own cream ground, the lockup box and the icons' background. */
export const GROUND = '#f9e5cd';
/** The hairline around the lockup box, the app's light border color. */
export const EDGE = '#e3d7c3';
/** The wordmark. */
export const INK = '#7c3d0a';

/** The favicon book's colors: outline, shade, cover, wood, ribbon and its shade, pages. */
export const BOOK = new Set([
	'#3b1d06',
	'#8a4210',
	'#a8560b',
	'#c47a30',
	'#86a86f',
	'#6f8f5c',
	'#f1e6d2'
]);
