import { resolve } from '$app/paths';
import type { Path, ResolvedPathname } from '$app/types';

/**
 * `resolve()` for an internal path known only at runtime: a `?next=`, the
 * target of a navigation. TypeScript checks a union argument against
 * `resolve`'s per-route signatures only while the app has at most 25 routes;
 * past that, `resolve(x as Path)` fails to type-check at every caller.
 */
export function resolvePath(path: Path): ResolvedPathname {
	return (resolve as (path: string) => ResolvedPathname)(path);
}
