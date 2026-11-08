<script lang="ts">
	import Globe from '@lucide/svelte/icons/globe';
	import Lock from '@lucide/svelte/icons/lock';
	import { Popover } from 'bits-ui';
	import { MediaQuery } from 'svelte/reactivity';
	import type { Person, Recipe } from '#lib/api/recipes.js';
	import type { PublicShare } from '#lib/api/shares.js';
	import { session } from '#lib/auth.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import { lockNotice } from '#lib/recipe/access.js';
	import { hasBeenEdited } from '#lib/recipe/authorship.js';
	import { formatDate } from '#lib/recipe/format.js';
	import BottomSheet from '#lib/components/ui/BottomSheet.svelte';
	import PersonCard from '#lib/components/ui/PersonCard.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';

	let {
		recipe,
		share = null,
		onmanageshare
	}: {
		recipe: Pick<Recipe, 'createdBy' | 'createdAt' | 'updatedBy' | 'updatedAt' | 'locked'>;
		/** The viewer's own public link to this recipe, if any: its line says
		 * how long it runs, or why it does not, and opens the pass-on sheet. */
		share?: PublicShare | null;
		onmanageshare?: () => void;
	} = $props();

	const edited = $derived(hasBeenEdited(recipe));

	// The sentence is translated whole, then split where the person and the
	// date go, so each language keeps its own order (as ProfileHead does).
	const WHO = '\u0000';
	const WHEN = '\u0001';
	const split = (text: string) =>
		text
			.split(WHO)
			.flatMap((chunk, i) => [
				...(i > 0 ? [WHO] : []),
				...chunk.split(WHEN).flatMap((part, j) => (j > 0 ? [WHEN, part] : [part]))
			])
			.filter(Boolean);
	const lines = $derived([
		{
			key: 'created',
			person: recipe.createdBy,
			date: formatDate(recipe.createdAt),
			parts: split(m.detail_created_by({ user: WHO, date: WHEN }))
		},
		...(edited
			? [
					{
						key: 'updated',
						person: recipe.updatedBy,
						date: formatDate(recipe.updatedAt),
						parts: split(m.detail_updated_by({ user: WHO, date: WHEN }))
					}
				]
			: [])
	]);

	// Which line's person card is open: a popover from md, a bottom sheet
	// below it, for the reason AuthorInitials gives. The sheet keeps its
	// person while it slides away.
	let open = $state<string | null>(null);
	let sheetOpen = $state(false);
	let sheetPerson = $state<Person | null>(null);
	const phone = new MediaQuery('max-width: 767.98px');
	// The line is caption text, 20px tall: an `::after` grows the target to
	// 44px around it without moving a pixel of the sentence.
	const trigger =
		'relative rounded-sm after:absolute after:inset-x-0 after:top-1/2 after:h-11 after:-translate-y-1/2 font-medium whitespace-nowrap text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary';
	const notice = $derived(lockNotice(recipe, session.user?.id));
	const shareLine = $derived.by(() => {
		if (!share) return '';
		if (share.status === 'paused') return m.detail_public_share_paused();
		if (share.status === 'limited') return m.detail_public_share_limited();
		return share.expiresAt
			? m.detail_public_share_until({ date: formatDate(share.expiresAt) })
			: m.detail_public_share_permanent();
	});
</script>

<!--
	Each person is one control: their mark and their name, which opens their
	card. The mark is decoration - the name is the control's name.
-->
{#snippet face(person: Person)}
	<span class="-my-1 mr-1.5 inline-block align-[-4.5px]"><PersonMark {person} size="xs" /></span
	><span
		class="underline decoration-border decoration-1 underline-offset-4 group-hover:decoration-current"
		>{person.displayName}</span
	>
{/snippet}

{#snippet who(key: string, person: Person)}
	{#if phone.current}
		<button
			type="button"
			class="group {trigger}"
			onclick={() => {
				sheetPerson = person;
				sheetOpen = true;
			}}
		>
			{@render face(person)}
		</button>
	{:else}
		<Popover.Root bind:open={() => open === key, (value) => (open = value ? key : null)}>
			<Popover.Trigger openOnHover openDelay={300} class="group {trigger}">
				{@render face(person)}
			</Popover.Trigger>
			<Popover.Portal>
				<!-- `inverse`, like the author panel on a card. -->
				<Popover.Content
					side="bottom"
					sideOffset={6}
					align="start"
					collisionPadding={12}
					class="z-50 w-80 max-w-[calc(100vw-24px)] rounded-2xl bg-inverse p-3.5 text-caption text-inverse-foreground shadow-dialog"
				>
					<PersonCard {person} compact inverse onnavigate={() => (open = null)} />
				</Popover.Content>
			</Popover.Portal>
		</Popover.Root>
	{/if}
{/snippet}

<footer class="mt-10 border-t border-border pt-6 text-caption text-text-muted">
	{#each lines as line, i (line.key)}
		<p class={i > 0 ? 'mt-2' : ''}>
			{#each line.parts as part, j (j)}
				{#if part === WHO}
					{@render who(line.key, line.person)}
				{:else if part === WHEN}
					<!-- A date never breaks inside: "4. November 2026" moves as one. -->
					<span class="whitespace-nowrap">{line.date}</span>
				{:else}
					{part}
				{/if}
			{/each}
		</p>
	{/each}
	{#if notice}
		<p class="mt-2 flex items-center gap-1.5">
			<Lock class="size-3.5 shrink-0" aria-hidden="true" />
			{notice}
		</p>
	{/if}
	{#if shareLine}
		<p class="mt-2 flex flex-wrap items-center gap-x-1.5">
			<Globe class="size-3.5 shrink-0 text-primary" aria-hidden="true" />
			<span class="font-semibold text-text">{shareLine}</span>
			{#if onmanageshare}
				<span aria-hidden="true">·</span>
				<button
					type="button"
					onclick={onmanageshare}
					class="font-semibold text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					{m.detail_public_share_manage()}
				</button>
			{/if}
		</p>
	{/if}
</footer>

{#if phone.current}
	<BottomSheet bind:open={sheetOpen} closeLabel={m.common_close()}>
		<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
			{#if sheetPerson}
				<PersonCard person={sheetPerson} asTitle onnavigate={() => (sheetOpen = false)} />
			{/if}
		</div>
	</BottomSheet>
{/if}
