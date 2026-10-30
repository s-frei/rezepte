import changelog from '@/content/(releases)/changelog/meta.json';

// "New" marks follow the newest release by themselves: a page sets
// `new: v1.3.0` in its frontmatter, a section puts <NewIn release="v1.3.0" />
// straight after its heading text (no space, or the heading's anchor gains a
// trailing hyphen). Both show only while that release is the newest, so
// nothing has to be removed when the next one ships. Read from the
// changelog's meta.json rather than the loader, so MDX can import this without
// importing the source that compiles it.

const RELEASES = changelog.pages.filter((page) => /^v\d+\.\d+\.\d+/.test(page));

/** The newest release, newest first in meta.json; release candidates are not announced. */
export const LATEST_RELEASE = RELEASES.find((version) => !version.includes('-'));

/** Whether a page or a section marked new in `release` shows its mark. A version with no changelog page fails the build. */
export function isNewIn(release: string | undefined): boolean {
	if (!release) return false;
	if (!RELEASES.includes(release)) throw new Error(`"new" names ${release}, which has no page in content/(releases)/changelog/`);
	return release === LATEST_RELEASE;
}

export function NewBadge({ className = '' }: { className?: string }) {
	return (
		<span
			className={`inline-block rounded-full bg-fd-primary px-2 py-0.5 align-middle font-sans text-[11px] leading-none font-semibold text-fd-primary-foreground ${className}`}
			title={`New in ${LATEST_RELEASE}`}
		>
			New
		</span>
	);
}

/** The mark after a heading: shown in the heading and in the table of contents while `release` is the newest. */
export function NewIn({ release }: { release: string }) {
	return isNewIn(release) ? <NewBadge className="ms-[0.5em]" /> : null;
}
