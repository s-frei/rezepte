import { Banner } from 'fumadocs-ui/components/banner';
import Link from 'next/link';
import { LATEST_RELEASE } from '@/components/new-in';
import { source } from '@/lib/source';

// How the changelog chapter presents a release: in the sidebar, as the
// timeline on the changelog page, and in a release page's heading.

const CHANGELOG_PREFIX = '/changelog/v';

// The release pages are titled "v1.3.0 — Rezepte as an app, timers that ring"
// (mise/lib/release/ writes them); the parts are shown at different sizes.
export function releaseParts(title: string): { version: string; name: string } | null {
	const m = /^(v\d+\.\d+\.\d+(?:-[\w.]+)?) — (.+)$/.exec(title);
	return m ? { version: m[1], name: m[2] } : null;
}

export const isReleaseUrl = (url: string) => url.startsWith(CHANGELOG_PREFIX);

/** The page of the newest release (see components/new-in.tsx). */
export function latestRelease() {
	return source.getPages().find((page) => page.url === `/changelog/${LATEST_RELEASE}`);
}

const releaseDate = new Intl.DateTimeFormat('en', { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'UTC' });

/** The sidebar entry of a release: the version large, its name beneath. */
export function ReleaseNavName({ title }: { title: string }) {
	const parts = releaseParts(title);
	if (!parts) return title;
	return (
		<span className="flex flex-col gap-0.5 py-0.5">
			<span className="font-display text-base font-semibold leading-tight">{parts.version}</span>
			<span className="text-xs leading-snug opacity-80">{parts.name}</span>
		</span>
	);
}

/** Every release, newest first, as a timeline. */
export function ReleaseTimeline() {
	const releases = source
		.getPages()
		.filter((page) => isReleaseUrl(page.url))
		.sort((a, b) => (b.data.date?.getTime() ?? 0) - (a.data.date?.getTime() ?? 0));
	return (
		<ol className="not-prose my-10 border-s-2 border-fd-border ps-0">
			{releases.map((page, i) => {
				const parts = releaseParts(page.data.title) ?? { version: page.data.title, name: '' };
				return (
					<li key={page.url} className="relative ms-0 mb-12 ps-8 last:mb-0">
						<span
							aria-hidden
							className={`absolute -start-[9px] top-3 size-4 rounded-full border-2 border-fd-background ${i === 0 ? 'bg-fd-primary' : 'bg-fd-border'}`}
						/>
						<Link href={page.url} className="group block">
							<span className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
								<span className={`font-display text-4xl font-semibold tracking-tight ${i === 0 ? 'text-fd-primary' : ''}`}>{parts.version}</span>
								{page.data.date && (
									<time dateTime={page.data.date.toISOString().slice(0, 10)} className="text-sm text-fd-muted-foreground">
										{releaseDate.format(page.data.date)}
									</time>
								)}
							</span>
							<span className="mt-2 block font-display text-xl font-semibold group-hover:text-fd-primary">{parts.name}</span>
							<span className="mt-2 block max-w-[38em] leading-relaxed text-fd-muted-foreground">{page.data.description}</span>
						</Link>
					</li>
				);
			})}
		</ol>
	);
}

/**
 * A line above every page naming the newest release. Closing it is
 * remembered per release (the id carries the version), so it comes back
 * once, with the next one.
 */
export function ReleaseBanner() {
	const latest = latestRelease();
	const parts = latest && releaseParts(latest.data.title);
	if (!latest || !parts) return null;
	return (
		// The primary color sets it apart from the navbar, which wears the page's own.
		<Banner id={`release-${parts.version}`} className="bg-fd-primary text-fd-primary-foreground [&_button]:text-fd-primary-foreground [&_button:hover]:bg-black/10">
			<Link href={latest.url} className="truncate px-4 underline-offset-4 hover:underline">
				<span className="font-display font-semibold">Rezepte {parts.version.slice(1)}</span>
				<span className="opacity-90 max-sm:hidden">: {parts.name}</span>
			</Link>
		</Banner>
	);
}
