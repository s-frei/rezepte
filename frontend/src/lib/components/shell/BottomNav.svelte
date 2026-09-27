<script lang="ts">
	import BookOpen from '@lucide/svelte/icons/book-open';
	import ChevronUp from '@lucide/svelte/icons/chevron-up';
	import CircleUserRound from '@lucide/svelte/icons/circle-user-round';
	import Plus from '@lucide/svelte/icons/plus';
	import Search from '@lucide/svelte/icons/search';
	import { tick } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { afterNavigate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { palette } from '$lib/palette.svelte';
	import { m } from '$lib/paraglide/messages';
	import { navScroll, startNavScroll } from './nav-scroll';
	import YouSheet from './YouSheet.svelte';

	let youOpen = $state(false);
	let scroll = $state(startNavScroll(0));
	const mode = $derived(scroll.mode);
	let navEl = $state<HTMLElement>();

	// The bar only exists below `md`; a wide window has no use for the work.
	const phone = new MediaQuery('max-width: 767.98px');

	// The page the bar is on - what `aria-current` says. Search and You are
	// sheets, not pages; while one is open it takes the accent instead, so
	// the pill shows what the reader is looking at.
	const place = $derived(
		page.url.pathname === '/' ? 'recipes' : page.url.pathname.startsWith('/settings') ? 'you' : null
	);
	const current = $derived(youOpen ? 'you' : palette.open ? 'search' : place);

	function onScroll() {
		if (!phone.current) return;
		const y = window.scrollY;
		const next = navScroll(scroll, y, document.documentElement.scrollHeight - window.innerHeight);
		// Never shrink away what someone is using: an open sheet, or an item
		// keyboard focus rests on. Only keyboard focus - after a tap, focus
		// stays in the bar too (the expanded item, or the trigger a closed
		// sheet hands it back to), and a phone's scrolling never moves it, so
		// `:focus-within` would keep the bar open for the rest of the page.
		if (
			next.mode === 'minimized' &&
			(youOpen || palette.open || navEl?.querySelector(':focus-visible'))
		) {
			scroll = { ...next, mode: 'expanded' };
			return;
		}
		scroll = next;
	}

	// Brings the whole bar back and puts focus on the current place (or the
	// recipes), since the button that was pressed or focused is replaced.
	async function expand() {
		scroll = startNavScroll(window.scrollY);
		await tick();
		const target =
			navEl?.querySelector<HTMLElement>('[data-current]') ??
			navEl?.querySelector<HTMLElement>('a, button');
		target?.focus();
	}

	// A new page starts with the whole bar, measuring from where it lands.
	afterNavigate(() => {
		scroll = startNavScroll(window.scrollY);
	});

	const item =
		'flex h-11 items-center justify-center gap-2 rounded-pill transition-all duration-200 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary motion-reduce:transition-none';
	// The current place is spelled out beside its icon on a filled pill, the
	// others stay icons: where you are reads at a glance, and the names are
	// learned in passing without every item needing its label.
	const itemState = (id: string) =>
		current === id
			? 'bg-accent px-4 text-accent-foreground'
			: 'w-13 text-text-muted hover:text-text';
	const glass =
		'rounded-pill border border-border/70 bg-surface/96 shadow-card backdrop-blur-2xl backdrop-saturate-150';
</script>

<svelte:window onscroll={onScroll} />

<!--
	The phone's way around, kept small: three icons in a pill of frosted
	surface on the right - the page scrolls visibly under it - and the one
	action that makes something, New, as a round button of its own on the
	left, so the account sits where "you" usually does, at the far end. The
	current place is an accent pill carrying its icon and its name; the other
	two are icons alone (a book, a lens, a person), named for screen readers.
	New comes first in the markup too, so the tab order reads the way the bar
	looks.

	Reading down shrinks the pill to the current icon and a chevron that says
	it opens again, and New to 44px, so a recipe fills the screen; scrolling
	up, reaching an end, a tap or focus brings the whole bar back
	(`navScroll`). The swap is instant under reduced motion.
-->
<a
	href={resolve('/recipes/new')}
	aria-label={m.nav_new()}
	class="fixed bottom-3 left-3 z-30 flex items-center justify-center rounded-full bg-primary text-primary-foreground shadow-card transition-all duration-200 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary motion-reduce:transition-none md:hidden {mode ===
	'expanded'
		? 'size-13'
		: 'size-11'}"
>
	<Plus class={mode === 'expanded' ? 'size-6' : 'size-5'} aria-hidden="true" />
</a>

<nav
	bind:this={navEl}
	aria-label={m.nav_main()}
	class="fixed right-3 bottom-3 z-30 flex items-center {glass} transition-all duration-200 motion-reduce:transition-none md:hidden {mode ===
	'expanded'
		? 'h-13 gap-0.5 px-1'
		: 'h-12 px-0.5'}"
>
	{#if mode === 'expanded'}
		<a
			href={resolve('/')}
			aria-label={m.nav_recipes()}
			aria-current={place === 'recipes' ? 'page' : undefined}
			data-current={current === 'recipes' ? '' : undefined}
			class="{item} {itemState('recipes')}"
		>
			<BookOpen class="size-5.5" aria-hidden="true" />
			{#if current === 'recipes'}<span class="text-body-sm font-semibold">{m.nav_recipes()}</span
				>{/if}
		</a>
		<button
			type="button"
			aria-label={m.nav_search()}
			aria-haspopup="dialog"
			aria-expanded={palette.open}
			data-current={current === 'search' ? '' : undefined}
			onclick={() => (palette.open = true)}
			class="{item} {itemState('search')}"
		>
			<Search class="size-5.5" aria-hidden="true" />
			{#if current === 'search'}<span class="text-body-sm font-semibold">{m.nav_search()}</span
				>{/if}
		</button>
		<button
			type="button"
			aria-label={m.nav_you()}
			aria-haspopup="dialog"
			aria-expanded={youOpen}
			data-current={current === 'you' ? '' : undefined}
			onclick={() => (youOpen = true)}
			class="{item} {itemState('you')}"
		>
			<CircleUserRound class="size-5.5" aria-hidden="true" />
			{#if current === 'you'}<span class="text-body-sm font-semibold">{m.nav_you()}</span>{/if}
		</button>
	{:else}
		<button
			type="button"
			aria-label={m.nav_expand()}
			onclick={expand}
			onfocus={expand}
			class="flex h-11 items-center justify-center gap-1 rounded-pill pr-2.5 pl-3.5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {current
				? 'bg-accent text-accent-foreground'
				: 'text-text-muted'}"
		>
			{#if current === 'you'}
				<CircleUserRound class="size-5" aria-hidden="true" />
			{:else if current === 'search'}
				<Search class="size-5" aria-hidden="true" />
			{:else}
				<BookOpen class="size-5" aria-hidden="true" />
			{/if}
			<!-- The pill folds open again; the chevron is what says so. -->
			<ChevronUp class="size-4 opacity-70" aria-hidden="true" />
		</button>
	{/if}
</nav>

<YouSheet bind:open={youOpen} />
