import { HomeLayout } from 'fumadocs-ui/layouts/home';
import type { Metadata } from 'next';
import Link from 'next/link';
import { Fragment, type CSSProperties } from 'react';
import { baseOptions } from '@/lib/layout.shared';
import { OG_SHARED, SITE_DESCRIPTION, SITE_TITLE, SITE_URL } from '@/lib/shared';

// The heading is a sentence, not the name; search results get the site title
// whole, as the template would turn it into "… — Rezepte — Rezepte".
export const metadata: Metadata = {
	title: { absolute: SITE_TITLE },
	description: SITE_DESCRIPTION,
	alternates: { canonical: SITE_URL },
	openGraph: { ...OG_SHARED, title: SITE_TITLE, description: SITE_DESCRIPTION, url: SITE_URL }
};

// Structured data for a software result in search: what Rezepte is, that it
// is free, and where it comes from.
const JSON_LD = {
	'@context': 'https://schema.org',
	'@type': 'SoftwareApplication',
	name: 'Rezepte',
	description: SITE_DESCRIPTION,
	url: SITE_URL,
	image: `${SITE_URL}og.png`,
	applicationCategory: 'LifestyleApplication',
	operatingSystem: 'Linux, macOS, Docker',
	license: 'https://www.apache.org/licenses/LICENSE-2.0',
	offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
	sameAs: ['https://github.com/s-frei/rezepte']
};

const shot = (name: string, device: 'desktop' | 'mobile', scheme: 'light' | 'dark') =>
	`/screenshots/${name}-${device}-${scheme}.png`;

type Tile = {
	title: string;
	text: string;
	href: string;
	/** A screenshot name from screenshots/; the tile shows its desktop and mobile variants. */
	shot: string;
	/** Columns of the six-column grid on wide screens. */
	span: 2 | 3 | 4;
	/** Shows the phone screenshot on wide screens too, in a narrow phone-shaped frame. */
	phone?: boolean;
	/** Two rows tall on wide screens. */
	tall?: boolean;
	/** What part of the desktop screenshot stays in view, as an object-position. */
	focus?: string;
	/** Zoom into the desktop screenshot around `focus`; a 1440px page is unreadable at tile size. */
	zoom?: number;
	/** What part of the mobile screenshot stays in view. */
	focusMobile?: string;
};

const SPAN = { 2: 'lg:col-span-2', 3: 'lg:col-span-3', 4: 'lg:col-span-4' } as const;

// The features worth a picture. Order and spans fill the six-column grid
// without gaps: cook mode takes the left column of the first two rows.
const TILES: Tile[] = [
	{ title: 'Cook mode with timers', text: 'Big type, one step at a time, and timers that run while you cook.', href: '/guide/cook-mode', shot: 'cook-mode-timers', span: 2, phone: true, tall: true, focus: 'center bottom', focusMobile: 'center bottom' },
	{ title: 'Import from anywhere', text: 'Paste a link or copied text, check the draft, save.', href: '/guide/importing', shot: 'import-dialog', span: 4, focus: 'center 52%', focusMobile: 'center bottom' },
	{ title: 'Search, filters and tags', text: 'Find tonight’s dinner by text, tag, time or author.', href: '/guide/tags-and-search', shot: 'active-filters', span: 2, focus: '4% 12%', zoom: 1.9 },
	{ title: 'A calm editor', text: 'Ingredient groups, steps, times and tag suggestions.', href: '/guide/recipes', shot: 'editor-ingredients', span: 2, focus: '50% 22%', zoom: 2 },
	{ title: 'Talk to your cookbook', text: 'Connect Claude or any MCP client to search, read and edit.', href: '/api/mcp', shot: 'token-mcp', span: 3, focus: '50% 40%', zoom: 1.5 },
	{ title: 'Notifications', text: 'New recipes of the day by push or mail, at your hour.', href: '/guide/notifications', shot: 'notifications-push', span: 3, focus: '60% 0%', zoom: 1.5 },
	{ title: 'Public links', text: 'Share a recipe with anyone, no account needed.', href: '/guide/sharing#public-links', shot: 'public-page', span: 3, focus: '60% top' },
	{ title: 'Install it as an app', text: 'On the home screen, and share links straight into it.', href: '/guide/install-app', shot: 'install-banner', span: 3, phone: true }
];

const IDEAS = [
	{ title: 'Servings and shopping list', text: 'Every quantity scales; copy the list with one tap.', href: '/guide/servings' },
	{ title: 'Comments', text: 'Write down what you learned for next time.', href: '/guide/comments' },
	{ title: 'Photos', text: 'Several per recipe, a cover and a lightbox.', href: '/guide/images' },
	{ title: 'Tasty marks', text: 'See who in the household liked what, at a glance on every card.', href: '/guide/recipes#tasty' },
	{ title: 'Editing rights', text: 'Decide who may change whose recipes.', href: '/guide/editing-rights' },
	{ title: 'Previews in your chats', text: 'A recipe link in WhatsApp or Signal shows its photo and title.', href: '/guide/sharing#the-link-preview' },
	{ title: 'Share as an image', text: 'Pass a recipe on to any messenger as one picture.', href: '/guide/sharing#pass-on-as-an-image' },
	{ title: 'Invite with a link', text: 'Bring the household in with a setup link, sent by mail if you like.', href: '/guide/users' },
	{ title: 'English and German', text: 'Every member picks their own language.', href: '/guide/settings' }
];

const DEMO_COMMAND = 'docker run --rm -p 8060:8060 ghcr.io/s-frei/rezepte:latest --demo';

const SELF_HOSTING = [
	'A single binary with the web app built in',
	'SQLite and your photos in one data directory',
	'Passwords, or single sign-on with your own provider',
	'A documented HTTP API with tokens for your scripts'
];

const eyebrow = 'mb-3 font-display text-lg italic text-fd-primary';
const card = 'rounded-2xl border border-fd-border bg-fd-card';

/** The screenshot in a tile, light and dark, desktop and mobile. */
function TileShot({ tile }: { tile: Tile }) {
	// The values differ per tile, so they reach the classes as variables.
	const style = {
		'--focus': tile.focus ?? 'top center',
		'--focus-mobile': tile.focusMobile ?? 'top center',
		'--zoom': tile.zoom ?? 1
	} as CSSProperties;
	// A phone screenshot sits in a phone-shaped frame cut off at the tile's
	// bottom edge, a desktop one fills the tile below its text.
	const frame = tile.phone
		? `mt-auto aspect-[360/640] w-[min(80%,260px)] rounded-t-2xl ${tile.tall ? '' : 'lg:w-[min(80%,220px)]'}`
		: 'max-lg:mt-auto max-lg:aspect-[360/640] max-lg:w-[min(80%,260px)] max-lg:rounded-t-2xl lg:min-h-[200px] lg:w-[calc(100%-44px)] lg:flex-1 lg:rounded-t-xl';
	return (
		<div className={`relative mx-auto overflow-hidden border border-b-0 border-fd-border ${frame}`}>
			{(['light', 'dark'] as const).map((scheme) => (
				<picture key={scheme} className={scheme === 'light' ? 'dark:hidden' : 'hidden dark:block'}>
					{!tile.phone && <source media="(min-width: 1024px)" srcSet={shot(tile.shot, 'desktop', scheme)} />}
					<img
						src={shot(tile.shot, 'mobile', scheme)}
						alt=""
						loading="lazy"
						style={style}
						className="absolute inset-0 size-full object-cover [object-position:var(--focus-mobile)] lg:[object-position:var(--focus)] lg:[transform-origin:var(--focus)] lg:[transform:scale(var(--zoom))]"
					/>
				</picture>
			))}
		</div>
	);
}

export default function Home() {
	return (
		<HomeLayout
			{...baseOptions()}
			links={[
				{ text: 'Docs', url: '/overview' },
				{ text: 'Changelog', url: '/changelog' }
			]}
		>
			<main className="mx-auto w-full max-w-[1120px] px-6 max-md:px-4">
				{/* A constant of our own, so the inline script holds nothing a reader could inject. */}
				<script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(JSON_LD) }} />
				<section className="grid items-center gap-10 pt-10 max-lg:pt-4 lg:grid-cols-[1fr_1.15fr]">
					<div>
						<p className={eyebrow}>Self-hosted recipe manager</p>
						<h1 className="font-display text-[clamp(2.1rem,5.6vw,3.7rem)] font-semibold leading-[1.06] tracking-tight">
							Your recipes. Your kitchen. Your server.
						</h1>
						<p className="mt-5 max-w-[36em] text-lg leading-relaxed text-fd-muted-foreground">
							Rezepte keeps the recipes of your household and your friends in one modern app, and makes cooking from them a pleasure.
						</p>
						<div className="mt-7 flex flex-wrap gap-3">
							<Link href="/getting-started" className="rounded-full bg-fd-primary px-5 py-3 font-bold text-fd-primary-foreground max-[400px]:flex-1 max-[400px]:text-center">
								Get started →
							</Link>
							<Link href="/guide" className="rounded-full border border-fd-border bg-fd-card px-5 py-3 font-bold max-[400px]:flex-1 max-[400px]:text-center">
								Take the tour
							</Link>
						</div>
					</div>
					{/* eslint-disable @next/next/no-img-element -- plain files from public/, no optimizer in a static export */}
					<div>
						<img src="/heroes/laptop-light.jpg" alt="The cookbook mascot presenting the recipe overview on a laptop" className="w-full rounded-2xl border border-fd-border shadow-lg dark:hidden" />
						<img src="/heroes/laptop-dark.jpg" alt="The cookbook mascot presenting the recipe overview on a laptop" className="hidden w-full rounded-2xl border border-fd-border shadow-lg dark:block" />
					</div>
					{/* eslint-enable @next/next/no-img-element */}
				</section>

				<section className="pt-22 max-lg:pt-16">
					<h2 className="mb-6 max-w-[18em] font-display text-[clamp(1.6rem,3.6vw,2.4rem)] font-semibold leading-tight tracking-tight">
						Collect them, cook them, share them
					</h2>
					<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-6">
						{TILES.map((tile) => (
							<Link
								key={tile.shot}
								href={tile.href}
								className={`${card} flex flex-col overflow-hidden transition-colors hover:border-fd-primary ${SPAN[tile.span]} ${tile.tall ? 'lg:row-span-2' : ''}`}
							>
								<div className="px-[22px] pt-[22px] pb-4">
									<h3 className="font-display text-xl font-semibold">{tile.title}</h3>
									<p className="mt-1.5 leading-relaxed text-fd-muted-foreground">{tile.text}</p>
								</div>
								<TileShot tile={tile} />
							</Link>
						))}
					</div>
				</section>

				<section className="pt-22 max-lg:pt-16">
					<h2 className="mb-6 font-display text-[clamp(1.6rem,3.6vw,2.4rem)] font-semibold leading-tight tracking-tight">And the small things that add up</h2>
					<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
						{IDEAS.map((idea) => (
							<Link key={idea.title} href={idea.href} className={`${card} p-[22px] transition-colors hover:border-fd-primary md:last:odd:col-span-2 lg:last:odd:col-span-1`}>
								<h3 className="font-display text-xl font-semibold">{idea.title}</h3>
								<p className="mt-1.5 leading-relaxed text-fd-muted-foreground">{idea.text}</p>
							</Link>
						))}
					</div>
				</section>

				<section className="pt-22 max-lg:pt-16">
					<div className={`${card} grid gap-x-8 gap-y-6 rounded-3xl p-10 max-lg:p-6 lg:grid-cols-2 lg:items-end`}>
						<div>
							<p className={eyebrow}>Yours, on your server</p>
							<h2 className="font-display text-[clamp(1.6rem,3.6vw,2.4rem)] font-semibold leading-tight tracking-tight">One image. One folder. Done.</h2>
						</div>
						<ul className="list-disc ps-5 leading-loose text-fd-muted-foreground marker:text-fd-primary">
							{SELF_HOSTING.map((line) => (
								<li key={line}>{line}</li>
							))}
						</ul>
						<div className="lg:col-span-2">
							<p className="mb-2.5 text-fd-muted-foreground">Try it with demo recipes:</p>
							<div className="flex flex-wrap items-center gap-x-6 gap-y-5">
								{/* Whole words only: where it has to wrap, it wraps at a space, never inside the image name, which
								    is why the type shrinks on the narrowest phones rather than letting the name break. */}
								<code className="min-w-0 flex-1 basis-[32rem] rounded-md border border-fd-border bg-fd-background px-3.5 py-3 font-mono text-[13px] leading-relaxed max-[359px]:text-[11px]">
									{DEMO_COMMAND.split(' ').map((word, i) => (
										<Fragment key={i}>
											{i > 0 && ' '}
											<span className="whitespace-nowrap">{word}</span>
										</Fragment>
									))}
								</code>
								<Link href="/getting-started/docker" className="rounded-full bg-fd-primary px-5 py-3 font-bold text-fd-primary-foreground">
									Install guide →
								</Link>
							</div>
						</div>
					</div>
				</section>
			</main>
			<footer className="mt-24 border-t border-fd-border py-8 text-sm text-fd-muted-foreground">
				<div className="mx-auto flex max-w-[1120px] flex-wrap justify-between gap-x-6 gap-y-3 px-6 max-md:px-4">
					<span>Rezepte · open source · Apache-2.0</span>
					<span className="flex gap-4">
						<Link href="/overview">Docs</Link>
						<Link href="/changelog">Changelog</Link>
						<Link href="/roadmap">Roadmap</Link>
						<a href="https://github.com/s-frei/rezepte">GitHub</a>
					</span>
				</div>
			</footer>
		</HomeLayout>
	);
}
