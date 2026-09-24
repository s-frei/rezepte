// A frontmatter string is one line of text, not MDX; backticks are the only
// markup it carries.
export function InlineCode({ text }: { text: string }) {
	return text.split(/(`[^`]+`)/).map((part, i) => (part.startsWith('`') ? <code key={i}>{part.slice(1, -1)}</code> : part));
}
