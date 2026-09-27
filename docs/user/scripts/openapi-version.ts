// The version the committed public/openapi.json carries. A running instance
// reports its build version as info.version - `git describe` between
// releases, so a new value with every commit - while the committed document
// names the release it was published with: release:start stamps that on the
// release branch (mise/lib/release/openapi.ts). Regenerating keeps whatever
// the committed document says, so CI's regenerate-and-diff check never sees
// the build version. See docs/memory/content/architecture/api.mdx.

type Doc = { info: { version: string } };

/** `fetched` with the committed document's info.version, or unchanged when there is none yet. */
export function keepCommittedVersion<T extends Doc>(fetched: T, committed: string | null): T {
	if (committed === null) return fetched;
	const version = (JSON.parse(committed) as Partial<Doc>).info?.version;
	if (typeof version !== 'string') return fetched;
	return { ...fetched, info: { ...fetched.info, version } };
}
