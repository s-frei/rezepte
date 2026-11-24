import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { BotIcon, HeartIcon, LanguagesIcon, MessageCircleIcon, SearchIcon, ShieldCheckIcon, TabletIcon, TimerIcon, UserPlusIcon } from 'lucide-react';
import type { Metadata } from 'next';
import Link from 'next/link';
import type { CSSProperties } from 'react';
import { CopyCommand } from '@/components/copy-command';
import { LandingFilm } from '@/components/landing-film';
import { LandingStage } from '@/components/landing-stage';
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
	/** A screenshot name from screenshots/; screenshots-audit.ts checks it has a test. */
	shot: string;
	/** Columns of the six-column grid on wide screens. */
	span: 2 | 4;
	/** Shows the phone screenshot on wide screens too, in a narrow phone-shaped frame. */
	phone?: boolean;
	/** What part of the desktop screenshot stays in view, as an object-position. */
	focus?: string;
	/** Zoom into the desktop screenshot around `focus`; a 1440px page is unreadable at tile size. */
	zoom?: number;
	/** What part of the mobile screenshot stays in view. */
	focusMobile?: string;
};

const SPAN = { 2: 'lg:col-span-2', 4: 'lg:col-span-4' } as const;

// The weekly loop, plus the way recipes get in. Spans 4+2 | 2+4 fill the six-column grid without gaps.
const BIG: Tile[] = [
	{ title: 'An editor that links it all', text: 'As you type, Rezepte suggests the ingredients and times in a step; link a photo to any step.', href: '/guide/recipes', shot: 'editor-step-photo', span: 4, focus: '72% 80%', zoom: 1.3 },
	{ title: 'Import from anywhere', text: 'Paste a link or copied text, or let an AI chat write the recipe; check the draft, save.', href: '/guide/importing', shot: 'import-dialog', span: 2, phone: true, focusMobile: 'center bottom' },
	{ title: 'Shop from one list', text: 'Put the week on a list in one go, sorted by department and ticked off live for everyone who shares it.', href: '/guide/shopping-lists', shot: 'list-shop', span: 2, phone: true },
	{ title: 'Plan the week', text: 'Put recipes and notes on the days of a week, alone or with the people you eat with, and see them in your calendar app.', href: '/guide/meal-plan', shot: 'plan-week', span: 4, focus: '0% 100%' }
];

// Three phone tiles, side by side from md up.
const MORE: Tile[] = [
	{ title: 'Install it as an app', text: 'Its own icon on the home screen and its own window, no browser bar.', href: '/guide/install-app', shot: 'install-banner', span: 2, phone: true },
	{ title: 'Share with anyone', text: 'A public link opens a recipe without an account, until it expires or you revoke it.', href: '/guide/sharing#public-links', shot: 'public-link-sheet', span: 2, phone: true, focusMobile: 'center 90%' },
	{ title: 'Notifications', text: 'Comments, invitations and the day’s new recipes, by push or mail.', href: '/guide/notifications', shot: 'notifications-push', span: 2, phone: true }
];

// Nine, so the three-column grid stays full.
const IDEAS = [
	{ icon: SearchIcon, title: 'Find tonight’s dinner', text: 'Search by text, tag, time or author, or type what you have and see what each recipe still needs.', href: '/guide/tags-and-search' },
	{ icon: TimerIcon, title: 'Timers side by side', text: 'Run timers across steps and recipes; they keep going after a reload.', href: '/guide/cook-mode#run-timers' },
	{ icon: TabletIcon, title: 'One tablet for the whole kitchen', text: 'Everyone stays signed in on the kitchen tablet; switch with a tap.', href: '/guide/settings#share-a-device' },
	{ icon: UserPlusIcon, title: 'Invite with a link', text: 'Admins send the people you cook with a setup link, by mail if you like.', href: '/guide/users#add-an-account' },
	{ icon: ShieldCheckIcon, title: 'Who may edit', text: 'Decide whether everyone may change a recipe, or only the people you choose.', href: '/guide/editing-rights' },
	{ icon: HeartIcon, title: 'Tasty marks and favorites', text: 'Hearts show who liked a recipe; stars mark your private favorites.', href: '/guide/recipes#tasty' },
	{ icon: MessageCircleIcon, title: 'Previews in your chats', text: 'Once the owner switches them on, a recipe link in a chat shows its photo and title.', href: '/guide/sharing#the-link-preview' },
	{ icon: BotIcon, title: 'API and MCP', text: 'Scripts and AI assistants such as Claude read and edit recipes, plans and lists.', href: '/api/mcp' },
	{ icon: LanguagesIcon, title: 'English and German', text: 'Everyone picks their own language.', href: '/guide/settings#choose-language-and-theme' }
];

const DEMO_PARTS = ['docker run --rm', '-p 8060:8060', 'ghcr.io/s-frei/rezepte:latest', '--demo'];
const DEMO_COMMAND = DEMO_PARTS.join(' ');

const SELF_HOSTING = [
	'A single binary with the web app built in',
	'SQLite and your photos in one data directory',
	'Passwords, or single sign-on with your own provider',
	'A documented HTTP API with tokens for your scripts'
];

const eyebrow = 'mb-3 font-display text-lg italic text-fd-primary';
const card = 'rounded-2xl border border-fd-border bg-fd-card';
const h2 = 'mb-6 font-display text-[clamp(1.6rem,3.6vw,2.4rem)] font-semibold leading-tight tracking-tight';

/* eslint-disable @next/next/no-img-element -- plain files from public/, no optimizer in a static export */

/** Light and dark of one screenshot, filling its positioned parent. */
function Shot({ name, device, style, className }: { name: string; device: 'desktop' | 'mobile'; style?: CSSProperties; className: string }) {
	return (
		<>
			{(['light', 'dark'] as const).map((scheme) => (
				<img key={scheme} src={shot(name, device, scheme)} alt="" loading="lazy" style={style} className={`absolute inset-0 size-full object-cover ${className} ${scheme === 'light' ? 'dark:hidden' : 'hidden dark:block'}`} />
			))}
		</>
	);
}

/* eslint-enable @next/next/no-img-element */

/** The screenshot in a tile: a phone frame below lg (and always for `phone`), the zoomed desktop shot from lg up. */
function TileShot({ tile, big }: { tile: Tile; big?: boolean }) {
	// The values differ per tile, so they reach the classes as variables.
	const style = { '--focus': tile.focus ?? 'top center', '--focus-mobile': tile.focusMobile ?? 'top center', '--zoom': tile.zoom ?? 1 } as CSSProperties;
	const frame = tile.phone
		? `mt-auto aspect-[360/640] w-[min(80%,260px)] rounded-t-2xl ${big ? 'lg:w-[min(80%,240px)]' : 'lg:w-[min(80%,220px)]'}`
		: `max-lg:mt-auto max-lg:aspect-[360/640] max-lg:w-[min(80%,260px)] max-lg:rounded-t-2xl lg:w-[calc(100%-44px)] lg:flex-1 lg:rounded-t-xl ${big ? 'lg:min-h-[320px]' : 'lg:min-h-[200px]'}`;
	return (
		<div className={`relative mx-auto overflow-hidden border border-b-0 border-fd-border ${frame}`}>
			{tile.phone ? (
				<Shot name={tile.shot} device="mobile" style={style} className="[object-position:var(--focus-mobile)]" />
			) : (
				<>
					<div className="lg:hidden">
						<Shot name={tile.shot} device="mobile" style={style} className="[object-position:var(--focus-mobile)]" />
					</div>
					<div className="max-lg:hidden">
						<Shot name={tile.shot} device="desktop" style={style} className="[object-position:var(--focus)] [transform-origin:var(--focus)] [transform:scale(var(--zoom))]" />
					</div>
				</>
			)}
		</div>
	);
}

/** `trio`: three tiles side by side from md up, so no tile is left alone in a row. */
function Bento({ tiles, big, trio }: { tiles: Tile[]; big?: boolean; trio?: boolean }) {
	return (
		<div className={`grid gap-4 ${trio ? 'md:grid-cols-3' : 'md:grid-cols-2 lg:grid-cols-6'}`}>
			{tiles.map((tile) => (
				<Link key={tile.shot} href={tile.href} className={`${card} flex flex-col overflow-hidden transition-colors hover:border-fd-primary ${trio ? '' : `md:max-lg:last:odd:col-span-2 ${SPAN[tile.span]}`}`}>
					<div className="px-[22px] pt-[22px] pb-4">
						<h3 className="font-display text-xl font-semibold">{tile.title}</h3>
						<p className="mt-1.5 leading-relaxed text-fd-muted-foreground">{tile.text}</p>
					</div>
					<TileShot tile={tile} big={big} />
				</Link>
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
				<section className="grid items-center gap-10 pt-10 max-lg:pt-4 max-lg:text-center lg:grid-cols-[1fr_1.15fr]">
					<div className="max-lg:flex max-lg:flex-col max-lg:items-center">
						<p className={eyebrow}>Self-hosted recipe manager</p>
						<h1 className="font-display text-[clamp(2.1rem,5.6vw,3.7rem)] font-semibold leading-[1.06] tracking-tight text-balance">
							Your recipes. Your kitchen. Your server.
						</h1>
						<p className="mt-5 max-w-[36em] text-lg leading-relaxed text-pretty text-fd-muted-foreground">
							Rezepte keeps everyone’s recipes in one app, cooks along step by step, plans the week and writes the shopping list.
						</p>
						<div className="mt-7 flex flex-wrap gap-3 max-lg:justify-center">
							<Link href="/getting-started" className="rounded-full bg-fd-primary px-5 py-3 font-bold whitespace-nowrap text-fd-primary-foreground max-[400px]:flex-1 max-[400px]:text-center max-[359px]:basis-full">
								Get started →
							</Link>
							<Link href="#tour" className="rounded-full border border-fd-border bg-fd-card px-5 py-3 font-bold whitespace-nowrap max-[400px]:flex-1 max-[400px]:text-center max-[359px]:basis-full">
								Take the tour
							</Link>
						</div>
					</div>
					<LandingStage />
				</section>

				<section id="tour" className="scroll-mt-20 pt-22 max-lg:pt-16">
					<div className="mx-auto mb-8 max-w-[40rem] text-center">
						<p className={eyebrow}>Take the tour</p>
						<h2 className="font-display text-[clamp(1.6rem,3.6vw,2.4rem)] font-semibold leading-tight tracking-tight text-balance">
							From the recipe to the shopping list
						</h2>
						<p className="mt-4 text-lg leading-relaxed text-pretty text-fd-muted-foreground">
							Scale it, edit it, cook it with a timer, talk about it and pass it on. Then plan it for the week and shop for it.
						</p>
					</div>
					<LandingFilm />
				</section>

				<section className="pt-22 max-lg:pt-16">
					<h2 className={`${h2} max-w-[18em]`}>Write it, plan it, shop it</h2>
					<Bento tiles={BIG} big />
				</section>

				<section className="pt-22 max-lg:pt-16">
					<h2 className={h2}>Runs the whole kitchen</h2>
					<Bento tiles={MORE} trio />
				</section>

				<section className="pt-22 max-lg:pt-16">
					<h2 className={h2}>And the small things that add up</h2>
					<div className="grid gap-4 md:grid-cols-3">
						{IDEAS.map((idea) => (
							<Link key={idea.title} href={idea.href} className={`${card} p-[22px] transition-colors hover:border-fd-primary`}>
								<idea.icon aria-hidden className="mb-3 size-6 text-fd-primary" strokeWidth={1.75} />
								<h3 className="font-display text-xl font-semibold">{idea.title}</h3>
								<p className="mt-1.5 leading-relaxed text-fd-muted-foreground">{idea.text}</p>
							</Link>
						))}
					</div>
				</section>

				<section className="pt-22 max-lg:pt-16">
					<div className={`${card} grid gap-x-8 gap-y-6 rounded-3xl p-10 max-lg:p-6 max-[359px]:p-4 lg:grid-cols-2 lg:items-end`}>
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
								<div className="@container relative flex min-w-0 flex-1 basis-[32rem] items-center gap-2 rounded-md border border-fd-border bg-fd-background py-1.5 ps-3.5 pe-1.5">
									<code className="min-w-0 flex-1 py-1.5 font-mono text-[13px] leading-relaxed @max-[34rem]:text-xs @max-[15rem]:text-[10px]">
										{/* Too narrow for one line: one part per line with a shell continuation that only CSS draws, so the copy button and a selection copy the one-line command. */}
										{DEMO_PARTS.map((part, i) => (
											<span key={part} className="whitespace-nowrap @max-[34rem]:not-first:ps-[2ch] @max-[34rem]:not-last:after:whitespace-pre @max-[34rem]:not-last:after:content-['\\\A']">
												{i < DEMO_PARTS.length - 1 ? `${part} ` : part}
											</span>
										))}
									</code>
									<CopyCommand command={DEMO_COMMAND} className="@max-[34rem]:absolute @max-[34rem]:top-1.5 @max-[34rem]:right-1.5" />
								</div>
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
