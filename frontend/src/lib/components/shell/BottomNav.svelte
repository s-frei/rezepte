<script lang="ts">
	import BookOpen from '@lucide/svelte/icons/book-open';
	import CircleUserRound from '@lucide/svelte/icons/circle-user-round';
	import Plus from '@lucide/svelte/icons/plus';
	import Search from '@lucide/svelte/icons/search';
	import { flushSync } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { afterNavigate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { palette } from '$lib/palette.svelte';
	import { m } from '$lib/paraglide/messages';
	import { bottomNav, type NavPlace } from '$lib/shell.svelte';
	import { docked } from './nav-scroll';
	import NewSheet from './NewSheet.svelte';
	import YouSheet from './YouSheet.svelte';

	let { place }: { place: NavPlace } = $props();

	let youOpen = $state(false);
	let newOpen = $state(false);
	// Set while a page arrives, so a page restored below the fold starts
	// docked instead of sliding down from the floating spot.
	let barEl = $state<HTMLElement>();
	bottomNav.docked = docked(window.scrollY);

	// The bar only exists below `md`; a wide window has no use for the work.
	const phone = new MediaQuery('max-width: 767.98px');
	const reducedMotion = new MediaQuery('prefers-reduced-motion: reduce');

	// What the reader is looking at: an open sheet takes the pill, and it
	// goes back to the place when the sheet closes.
	const current = $derived(youOpen ? 'you' : palette.open ? 'search' : place);
	const slot = $derived(current === 'recipes' ? 0 : current === 'search' ? 1 : 2);

	function onScroll() {
		if (phone.current) bottomNav.docked = docked(window.scrollY);
	}

	// The new shape is written with transitions off and read back once, so
	// the browser settles on it before they come back on - a frame callback
	// can run before the style is recalculated, and the slide would play.
	afterNavigate(() => {
		bottomNav.still = true;
		bottomNav.docked = docked(window.scrollY);
		flushSync();
		void barEl?.offsetHeight;
		bottomNav.still = false;
	});

	// On the overview, a tap while reading goes back to the top and keeps the
	// search and filters; only a tap at the top starts over at `/`.
	function onRecipes(event: MouseEvent) {
		if (place !== 'recipes' || window.scrollY === 0) return;
		event.preventDefault();
		window.scrollTo({ top: 0, behavior: reducedMotion.current ? 'auto' : 'smooth' });
	}

	const item =
		'relative flex h-12 flex-1 items-center justify-center rounded-pill transition-colors duration-200 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary motion-reduce:transition-none';
	const itemState = (id: string) =>
		current === id ? 'text-accent-foreground' : 'text-text hover:text-accent-foreground';
</script>

<svelte:window onscroll={onScroll} />

<!--
	The phone's way around: one bar of three icons - a book for the recipes, a
	lens for search, a person for you - and New, the one action that makes
	something, as a primary circle at its end. No word is printed in it, so no
	language can squeeze it: the three places are equal slots in every locale
	and on every page, and the names are the buttons' accessible names. The
	current place sits on an accent pill that glides between the slots.

	At the top of a page the bar floats like a control; once reading starts
	(`docked`) it settles onto the bottom edge, a step in from the sides with
	the sheets' rounded top, and lifts off again near the top. The bar always
	has the docked size; floating is that bar scaled down and lifted, so the
	morph moves no layout and stays smooth while the page scrolls under it.
	It never minimizes while reading. It steps aside while a field has focus,
	and `AppShell` only renders it where one of its places is current
	(`navPlace`).
	Its own view-transition name keeps it above the overview's cards while
	they glide to a new card size; unnamed, it would be part of the page's
	picture under them.
-->
<div
	bind:this={barEl}
	data-docked={bottomNav.docked ? '' : undefined}
	class="fixed inset-x-2 bottom-0 z-30 flex h-[calc(4rem+env(safe-area-inset-bottom))] origin-bottom items-center border border-border bg-surface px-2.5 pb-[env(safe-area-inset-bottom)] transition-[translate,scale,border-radius,border-color,box-shadow] duration-500 ease-[cubic-bezier(0.3,1.3,0.5,1)] [view-transition-name:bottom-nav] motion-reduce:transition-none md:hidden {bottomNav.docked
		? 'rounded-t-3xl border-b-transparent shadow-sheet'
		: '-translate-y-[calc(0.75rem+env(safe-area-inset-bottom))] scale-95 rounded-dock shadow-card'} {bottomNav.still
		? 'transition-none'
		: ''} {bottomNav.typing ? 'hidden' : ''}"
>
	<nav aria-label={m.nav_main()} class="relative flex flex-1 items-center">
		<span
			aria-hidden="true"
			class="absolute inset-y-0 left-0 w-1/3 rounded-pill bg-accent transition-transform duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none"
			style:transform="translateX({slot * 100}%)"
		></span>
		<a
			href={resolve('/')}
			aria-label={m.nav_recipes()}
			aria-current={place === 'recipes' ? 'page' : undefined}
			data-current={current === 'recipes' ? '' : undefined}
			onclick={onRecipes}
			class="{item} {itemState('recipes')}"
		>
			<BookOpen class="size-6" aria-hidden="true" />
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
			<Search class="size-6" aria-hidden="true" />
		</button>
		<button
			type="button"
			aria-label={m.nav_you()}
			aria-haspopup="dialog"
			aria-expanded={youOpen}
			aria-current={place === 'you' ? 'true' : undefined}
			data-current={current === 'you' ? '' : undefined}
			onclick={() => (youOpen = true)}
			class="{item} {itemState('you')}"
		>
			<CircleUserRound class="size-6" aria-hidden="true" />
		</button>
	</nav>
	<button
		type="button"
		aria-label={m.nav_new()}
		aria-haspopup="dialog"
		aria-expanded={newOpen}
		onclick={() => (newOpen = true)}
		class="ml-1.5 flex size-12 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-cta ring-2 ring-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
	>
		<Plus class="size-6" aria-hidden="true" />
	</button>
</div>

<YouSheet bind:open={youOpen} />
<NewSheet bind:open={newOpen} />
