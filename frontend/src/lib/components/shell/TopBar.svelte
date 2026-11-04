<script lang="ts">
	import Import from '@lucide/svelte/icons/import';
	import Plus from '@lucide/svelte/icons/plus';
	import { flushSync } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import wordmark from '$brand/lockups/rezepte-wordmark.svg?url';
	import Lockup from '#lib/components/brand/Lockup.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { importDialog } from '#lib/import.svelte.js';
	import { invitations } from '#lib/shopping/invitations.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { navPlace, shell } from '#lib/shell.svelte.js';
	import UserMenu from './UserMenu.svelte';

	// The same places the phone's bottom nav marks, so both read alike.
	const place = $derived(navPlace(page.route.id, page.error !== null));
	// Where the phone hides its bar - the detail page, the editors - the
	// segments go too: those pages bring their own actions, and the bar
	// cannot hold both.
	const showNav = $derived(place !== null);
	// The overview of lists is named by its own segment already.
	const breadcrumb = $derived(page.route.id === '/lists' ? undefined : shell.breadcrumb);

	const segment =
		'inline-flex h-8 items-center gap-2 rounded-pill px-4 text-body-sm font-semibold transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
	const segmentState = (current: boolean) =>
		current ? 'bg-accent text-accent-foreground' : 'text-text-muted hover:text-text';
	const round =
		'inline-flex size-10 items-center justify-center rounded-full transition hover-tint focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]';

	// "New recipe" turns into a circle, as on the phone, only while its label
	// does not fit beside the centered segments: measured rather than set at a
	// width, so it holds for every language and type size. `labeled` keeps the
	// row's labeled width while the circle shows, to know when it fits again.
	let compact = $state(false);
	let labeled = 0;
	function fitLabel(box: HTMLElement) {
		const row = box.firstElementChild as HTMLElement;
		const observer = new ResizeObserver(() => {
			if (!compact) labeled = row.offsetWidth;
			const next = labeled > box.clientWidth;
			if (next !== compact) {
				compact = next;
				// Before the frame is painted, so the label never shows cut.
				flushSync();
			}
		});
		observer.observe(box);
		observer.observe(row);
		return () => observer.disconnect();
	}
</script>

<!-- Where the segments are, they sit in the exact middle at every width: a
     flex row centered them between a narrow tab and the wide buttons, off
     center by 93px. The sides are equal however wide their contents, which
     is what the actions measure themselves against. Named for the page
     transition (app.css), so it holds still while the page under it
     changes. -->
<header
	class="mx-auto hidden h-16 max-w-[1280px] items-center justify-between gap-4 px-8 [view-transition-name:top-bar] {showNav
		? 'md:grid md:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]'
		: 'md:flex'}"
>
	<div class="flex min-w-0 items-center gap-4 self-stretch">
		<!-- A tab hanging from the window's top edge, like the book's ribbon:
		     card stock in both themes, where the lockup's cream box was the one
		     light block on a dark page. -->
		<a
			href={resolve('/')}
			aria-label={m.app_name()}
			class="flex shrink-0 items-center gap-2 self-start rounded-b-2xl border border-t-0 border-border bg-surface pt-3.5 pr-3 pb-2.5 pl-3 shadow-card focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary lg:pr-4.5"
		>
			<Lockup variant="mark" class="h-10" />
			<!-- Quoted: Vite inlines the small SVG as a data URI, whose parentheses
			     break a bare url(). Nudged down: the "p" descender makes the
			     centered box read high. Below lg the mascot stands alone: with
			     the wordmark the bar left 27px between its parts at 800px, and
			     "Recipes" sits right beside it in the segments anyway. -->
			<span
				aria-hidden="true"
				style:mask-image={`url("${wordmark}")`}
				class="block aspect-[386.8/94.6] h-6 translate-y-0.5 bg-accent-foreground mask-contain mask-center mask-no-repeat forced-color-adjust-none max-lg:hidden"
			></span>
		</a>
		{#if breadcrumb}
			<!-- Beside the segments it only fits from lg. -->
			<span class="truncate text-body-sm text-text-muted {showNav ? 'max-lg:hidden' : ''}">
				{breadcrumb}
			</span>
		{/if}
	</div>
	{#if showNav}
		<nav
			aria-label={m.nav_main()}
			class="flex shrink-0 gap-0.5 rounded-pill border border-border bg-surface p-1"
		>
			<a
				href={resolve('/')}
				aria-current={place === 'recipes' ? 'page' : undefined}
				class="{segment} {segmentState(place === 'recipes')}"
			>
				{m.nav_recipes()}
			</a>
			<a
				href={resolve('/lists')}
				aria-current={place === 'lists' ? 'page' : undefined}
				class="{segment} {segmentState(place === 'lists')}"
			>
				{m.nav_lists()}
				{#if invitations.count > 0}
					<span aria-hidden="true" class="size-2 rounded-full bg-primary"></span>
					<span class="sr-only">{invitations.label}</span>
				{/if}
			</a>
		</nav>
	{/if}
	<div {@attach fitLabel} class="flex min-w-0 shrink-0 justify-end">
		<div class="flex shrink-0 items-center gap-3">
			{#if shell.actions}
				{@render shell.actions()}
			{:else}
				<!-- A circle: importing is reached for now and then, while
				     "New recipe" beside it is what the bar is for. -->
				<button
					type="button"
					aria-label={m.overview_import()}
					title={m.overview_import()}
					class="{round} border border-border bg-surface text-text"
					onclick={() => (importDialog.open = true)}
				>
					<Import class="size-4" aria-hidden="true" />
				</button>
				{#if compact}
					<a
						href={resolve('/recipes/new')}
						aria-label={m.overview_new_recipe()}
						title={m.overview_new_recipe()}
						class="{round} bg-primary text-primary-foreground"
					>
						<Plus class="size-4" aria-hidden="true" />
					</a>
				{:else}
					<Button variant="primary" href={resolve('/recipes/new')}>
						<Plus class="size-4" aria-hidden="true" />
						{m.overview_new_recipe()}
					</Button>
				{/if}
			{/if}
			<UserMenu />
		</div>
	</div>
</header>
