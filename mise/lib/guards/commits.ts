// The commit message rules of docs/memory/content/conventions/commits.mdx, as
// a pure function so they test without a repository. `ci` and `build` are
// accepted beside the page's list because the history already uses them.
const TITLE = /^(feat|fix|docs|chore|refactor|test|style|ci|build)(\([a-z0-9][a-z0-9/._-]*\))?(!)?: \S/;
// Trailers a tool appends: Co-Authored-By, and "Generated with ..." lines
// with or without a leading emoji.
const TRAILER = /^(co-authored-by:|\W*generated (with|by) )/i;

export function violations(message: string): string[] {
	const [title = '', ...body] = message.trimEnd().split('\n');
	const found: string[] = [];
	const m = TITLE.exec(title);
	if (!m) found.push('title is not `type(scope): summary` with type feat, fix, docs, chore, refactor, test, style, ci or build');
	if (title.length > 70) found.push(`title is ${title.length} characters, at most 70`);
	if (m?.[3] && !body.some((l) => l.startsWith('Breaking: '))) found.push('a breaking `!` needs a body line starting `Breaking: `');
	for (const l of body) if (TRAILER.test(l)) found.push(`tool trailer \`${l.trim()}\``);
	return found;
}

// Fixups are folded away by `rebase --autosquash` before the branch lands.
export const isFixup = (message: string) => /^(fixup|squash|amend)! /.test(message);
