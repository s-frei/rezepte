<script lang="ts">
	import Import from '@lucide/svelte/icons/import';
	import Plus from '@lucide/svelte/icons/plus';
	import { flushSync } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import BrandTab from '#lib/components/brand/BrandTab.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { importDialog } from '#lib/import.svelte.js';
	import { listImport, startList } from '#lib/shopping/new-list.svelte.js';
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
	// The actions make something new where the segments say you are: a
	// recipe, or on the lists a list - as the phone's "+" does.
	const lists = $derived(place === 'lists');
	const importLabel = $derived(lists ? m.list_import_title() : m.overview_import());
	const newLabel = $derived(lists ? m.lists_new() : m.overview_new_recipe());

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
	// Another label has another width: measured afresh, labeled first.
	$effect(() => {
		void lists;
		compact = false;
	});
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
		<!-- Below lg the mascot stands alone: with the wordmark the bar left
		     27px between its parts at 800px, and "Recipes" sits right beside it
		     in the segments anyway. -->
		<BrandTab label={m.app_name()} href={resolve('/')} wordmarkClass="max-lg:hidden" />
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
				     "New recipe" or "New list" beside it is what the bar is for. -->
				<button
					type="button"
					aria-label={importLabel}
					title={importLabel}
					class="{round} border border-border bg-surface text-text"
					onclick={() => (lists ? (listImport.open = true) : (importDialog.open = true))}
				>
					<Import class="size-4" aria-hidden="true" />
				</button>
				{#if lists && compact}
					<button
						type="button"
						aria-label={newLabel}
						title={newLabel}
						data-new
						class="{round} bg-primary text-primary-foreground"
						onclick={startList}
					>
						<Plus class="size-4" aria-hidden="true" />
					</button>
				{:else if compact}
					<a
						href={resolve('/recipes/new')}
						aria-label={newLabel}
						title={newLabel}
						data-new
						class="{round} bg-primary text-primary-foreground"
					>
						<Plus class="size-4" aria-hidden="true" />
					</a>
				{:else}
					<Button
						variant="primary"
						href={lists ? undefined : resolve('/recipes/new')}
						onclick={lists ? startList : undefined}
						data-new
					>
						<Plus class="size-4" aria-hidden="true" />
						{newLabel}
					</Button>
				{/if}
			{/if}
			<UserMenu />
		</div>
	</div>
</header>
