// The list a quiet summary line opens into: a step's links (StepLinks),
// an import's tag suggestions (TagSuggestions), and the steps' "Accept all"
// row (StepEditor). Whole literals, because Tailwind reads this file as text.
//
// One grouped surface in the input fields' color, rows of one height split
// by hairlines: the circles get room above and below instead of touching a
// line, and the eye runs down one column of them. A row's text starts where
// the step field's does (`px-3.5`: its 1px border and 14px padding), and a
// phone gets 44px rows, the touch minimum, against 48px from `md:`.
export const LIST =
	'list-none divide-y divide-border overflow-hidden rounded-md border border-border bg-surface-elevated';
export const ROW =
	'grid min-h-11 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 px-3.5 py-1 text-body-sm md:min-h-12 md:py-1.5';
// A row's name and amount: the amount sits at the right end, and drops under
// the name once both no longer fit beside each other (a narrow phone at a
// large text size) instead of squeezing into a column of its own.
export const CELL = 'flex min-w-0 flex-wrap items-center justify-between gap-x-3';
// The quiet summary line that opens such a list (TagSuggestions, and the
// import dialog's example).
export const TOGGLE =
	'-mr-1 flex items-center gap-1 rounded-pill px-1 py-1 text-caption text-primary transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
// A row's buttons: separate circles, 32px to see on a phone and 36px from
// `md:`, 44px to hit either way (the `before:` inset), 12px apart so two
// targets never overlap.
export const ACTIONS = 'flex shrink-0 gap-3';
// No color here: ACCEPT or REMOVE says what a button does, by its icon. Both
// are quiet circles, so a list of proposals is not a column of color; the
// color comes on hover, the primary to take, the destructive to take away.
export const ACTION =
	"relative flex size-8 shrink-0 items-center justify-center rounded-full border border-border bg-surface-elevated transition before:absolute before:-inset-1.5 before:content-[''] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-40 md:size-9 md:before:-inset-1";
export const ACCEPT = 'text-text hover:border-primary hover:text-primary';
export const REMOVE = 'text-text-muted hover:border-destructive hover:text-destructive';
