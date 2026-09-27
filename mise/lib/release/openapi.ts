// The committed OpenAPI document's info.version. A running instance reports
// its build version there; the committed copy, which the user docs publish,
// carries the version of the release it was published with. release:start
// stamps it on the release branch and release:finish refuses a release whose
// document says anything else. docs/user/scripts/fetch-openapi.ts keeps the
// committed value when it regenerates the document, so a build between
// releases never changes it. See docs/memory/content/architecture/api.mdx.

export const OPENAPI_PATH = 'docs/user/public/openapi.json';

/** The tag `v1.2.0` as the binary and the document write it: `1.2.0`. */
export const specVersion = (tag: string) => tag.replace(/^v/, '');

type Doc = { info?: { version?: unknown } };

/** The document with info.version set to the release, formatted the way fetch-openapi.ts writes it. */
export function stampSpec(src: string, tag: string): string {
	const doc = JSON.parse(src) as Doc;
	if (!doc.info || typeof doc.info !== 'object') throw new Error(`${OPENAPI_PATH} has no info object`);
	doc.info.version = specVersion(tag);
	return JSON.stringify(doc, null, 2) + '\n';
}

/** Why the document does not carry the release's version, or null when it does. */
export function checkSpec(src: string, tag: string): string | null {
	let doc: Doc;
	try {
		doc = JSON.parse(src) as Doc;
	} catch (e) {
		return `${OPENAPI_PATH} is not JSON: ${(e as Error).message}`;
	}
	const want = specVersion(tag);
	const got = doc.info?.version;
	if (got === want) return null;
	return `${OPENAPI_PATH} says info.version ${JSON.stringify(got)}, not "${want}" - release:start stamps it; set it to "${want}" in the release worktree and commit it`;
}
