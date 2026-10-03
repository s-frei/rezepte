// The dashed list a quiet summary line opens into: a step's links
// (StepLinks) and an import's tag suggestions (TagSuggestions).
// Whole literals, because Tailwind reads this file as text.
export const ROW = 'grid grid-cols-[1fr_auto_auto] items-center gap-x-3 py-1 text-body-sm';
// The quiet summary line that opens such a list (TagSuggestions, and the
// import dialog's example).
export const TOGGLE =
	'-mr-1 flex items-center gap-1 rounded-pill px-1 py-1 text-caption text-primary transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
// No hover color here: each button says what it does with its own.
export const ACTION =
	'flex size-8 items-center justify-center rounded-pill text-text-muted transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
