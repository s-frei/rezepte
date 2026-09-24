// Release versions: which commits a release contains and which number it gets.
// Tags are the only record of released versions - there is no version file -
// so everything here works from the last tag and the commit subjects since
// it. See docs/memory/content/architecture/releases.mdx.

export type Classified = { breaking: string[]; features: string[]; other: string[] };

// `type(scope)!: summary` or `type!: summary` - the `!` must sit right before
// the colon, so an exclamation mark in the summary does not count.
const BREAKING = /^\w+(\([^)]*\))?!:/;
const FEATURE = /^feat(\([^)]*\))?:/;

export function classify(subjects: string[]): Classified {
	const c: Classified = { breaking: [], features: [], other: [] };
	for (const s of subjects) {
		if (BREAKING.test(s)) c.breaking.push(s);
		else if (FEATURE.test(s)) c.features.push(s);
		else c.other.push(s);
	}
	return c;
}

export type Version = { major: number; minor: number; patch: number; prerelease?: string };

const VERSION = /^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z.-]+))?$/;

export function parseVersion(tag: string): Version | null {
	const m = VERSION.exec(tag);
	if (!m) return null;
	const v: Version = { major: Number(m[1]), minor: Number(m[2]), patch: Number(m[3]) };
	if (m[4]) v.prerelease = m[4];
	return v;
}

export function formatVersion(v: Version): string {
	return `v${v.major}.${v.minor}.${v.patch}${v.prerelease ? `-${v.prerelease}` : ''}`;
}

export function compareVersions(a: Version, b: Version): number {
	const d = a.major - b.major || a.minor - b.minor || a.patch - b.patch;
	if (d !== 0) return d;
	// A prerelease precedes its release; two prereleases compare as text, which
	// is enough for the rc1, rc2 naming this project uses.
	if (a.prerelease === b.prerelease) return 0;
	if (!a.prerelease) return 1;
	if (!b.prerelease) return -1;
	return a.prerelease < b.prerelease ? -1 : 1;
}

function parseTag(tag: string): Version {
	const v = parseVersion(tag);
	if (!v) throw new Error(`the tag ${tag} is not a version`);
	return v;
}

// The step semver asks for after `release`, given what came since it.
function bump(release: Version, c: Classified): Version {
	if (c.breaking.length > 0) return { major: release.major + 1, minor: 0, patch: 0 };
	if (c.features.length > 0) return { major: release.major, minor: release.minor + 1, patch: 0 };
	return { major: release.major, minor: release.minor, patch: release.patch + 1 };
}

// nearestTag: the closest version tag, candidates included. lastReleaseTag:
// the closest one that is not a candidate. sinceRelease: the commits since
// lastReleaseTag - which is also what came since nearestTag whenever the two
// are the same tag.
export function suggest(nearestTag: string | null, lastReleaseTag: string | null, sinceRelease: Classified): string {
	if (nearestTag === null) return 'v1.0.0';
	const nearest = parseTag(nearestTag);
	if (nearest.prerelease) {
		// After a candidate the next step is the release it previews, even with
		// nothing new since - that is how a candidate gets promoted - unless
		// what landed since the last release needs a bigger step than that.
		const previewed: Version = { major: nearest.major, minor: nearest.minor, patch: nearest.patch };
		const needed = lastReleaseTag === null ? parseTag('v1.0.0') : bump(parseTag(lastReleaseTag), sinceRelease);
		return formatVersion(compareVersions(needed, previewed) > 0 ? needed : previewed);
	}
	if (sinceRelease.breaking.length + sinceRelease.features.length + sinceRelease.other.length === 0) {
		throw new Error(`nothing to release since ${nearestTag}`);
	}
	return formatVersion(bump(nearest, sinceRelease));
}

// Which tag a release's commits are counted from: a release covers everything
// since the previous release, so a v2.0.0 after v2.0.0-rc1 still lists - and
// still demands the upgrade notice for - a breaking change that shipped in
// the candidate. A candidate covers only what came since the nearest tag.
export function classificationBase(version: string, nearestTag: string | null, lastReleaseTag: string | null): string | null {
	return parseVersion(version)?.prerelease ? nearestTag : lastReleaseTag;
}

// A warning, not a refusal: the maintainer may know that a `!` commit breaks
// nothing anyone runs. A candidate counts as the release it previews, so
// v2.0.0-rc1 is not a smaller step than v2.0.0.
export function belowSuggestion(version: string, suggestion: string): string | null {
	const release = (v: Version): Version => ({ major: v.major, minor: v.minor, patch: v.patch });
	const chosen = release(parseTag(version));
	if (compareVersions(chosen, release(parseTag(suggestion))) >= 0) return null;
	return `${version} is a smaller step than the commits since the last release ask for (${suggestion}) - readers going by semver will expect less change than it carries`;
}

export function validate(version: string, lastTag: string | null): string | null {
	const v = parseVersion(version);
	if (!v) return `"${version}" is not a version like v1.2.3 or v1.2.3-rc1`;
	const last = lastTag === null ? null : parseVersion(lastTag);
	if (last && compareVersions(v, last) <= 0) return `${version} is not greater than the last release ${lastTag}`;
	return null;
}
