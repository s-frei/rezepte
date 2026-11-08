<script lang="ts">
	import X from '@lucide/svelte/icons/x';
	import { Combobox } from 'bits-ui';
	import { tick } from 'svelte';
	import { flip } from 'svelte/animate';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { scale } from 'svelte/transition';
	import type { PersonEntry } from '#lib/api/users.js';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * The share dialog's picker: a search field whose picks sit in it as
	 * chips. Focusing it suggests everyone in `people` at once (a household
	 * has a handful, nobody should have to type); typing narrows by display
	 * name and username. Every row and chip carries the person's picture and
	 * username, so two people with one first name tell apart. Picking sends
	 * nothing - the dialog's button does.
	 */
	let {
		people,
		picked = $bindable([]),
		failed,
		disabled = false
	}: {
		/** Who may be invited: not on the list yet, not the viewer. */
		people: PersonEntry[];
		/** Ids of the chosen people, in the order they were picked. */
		picked?: string[];
		/** Ids whose invitation failed; their chips say so. */
		failed: Set<string>;
		/** While the invitations are out. */
		disabled?: boolean;
	} = $props();

	// A picked chip grows in and the chips after it make room, as the
	// overview's filter chips do. Local, so the field never pops on open.
	const flipDuration = $derived(prefersReducedMotion.current ? 0 : 200);
	const grow = $derived({
		start: prefersReducedMotion.current ? 1 : 0.9,
		duration: prefersReducedMotion.current ? 0 : 160,
		easing: cubicOut
	});

	let open = $state(false);
	let query = $state('');
	let input = $state<HTMLInputElement | null>(null);

	// Lowercased without accents, so "zoe" finds "Zoë".
	const fold = (s: string) =>
		s
			.normalize('NFD')
			.replace(/\p{Diacritic}/gu, '')
			.toLowerCase();

	const byId = $derived(new Map(people.map((p) => [p.id, p])));
	const chips = $derived(picked.map((id) => byId.get(id)).filter((p) => p !== undefined));
	const suggestions = $derived.by(() => {
		const q = fold(query.trim());
		return people.filter(
			(p) =>
				!picked.includes(p.id) &&
				(!q || fold(p.displayName).includes(q) || fold(p.username).includes(q))
		);
	});
	const none = $derived(people.length === 0);
	/** Display names more than one person carries: their chips add the username. */
	const shared = $derived(
		new Set(people.map((p) => p.displayName).filter((n, i, all) => all.indexOf(n) !== i))
	);

	/** What a screen reader hears about the picks: the chip itself is silent. */
	let said = $state('');

	function drop(id: string) {
		picked = picked.filter((p) => p !== id);
		const person = byId.get(id);
		if (person) said = m.list_share_unpicked({ name: person.displayName });
	}

	function unpick(id: string) {
		drop(id);
		input?.focus();
	}

	// Bits writes the picked label into its own copy of the field's text,
	// which `inputValue` only overrides when it changes: hand it the label
	// and then the empty string, so its copy is empty too (otherwise a
	// Backspace on the empty field reopened the list). The field is for the
	// next search; the pick shows as its chip.
	async function cleared(next: string[]) {
		const person = byId.get(next[next.length - 1] ?? '');
		if (person) said = m.list_share_picked({ name: person.displayName });
		query = person?.displayName ?? '';
		await tick();
		query = '';
		await tick();
		if (input) input.value = '';
		// The field caps its height and scrolls; keep the typing line in view.
		input?.scrollIntoView({ block: 'nearest' });
	}

	function onkeydown(event: KeyboardEvent) {
		if (event.key === 'Backspace' && query === '' && picked.length > 0) {
			drop(picked[picked.length - 1]);
		}
	}
</script>

<Combobox.Root
	type="multiple"
	bind:value={picked}
	bind:open
	inputValue={query}
	disabled={disabled || none}
	onValueChange={cleared}
>
	<!--
		The field: chips, then the input, wrapping onto more lines as picks grow,
		up to about three lines; beyond that it scrolls inside, so on a phone
		the suggestions under it stay on screen however many are picked.
		The list opens on click, never on focus: the dialog is centered, so a
		list opened on pointerdown grows it and moves the field up, and the
		pointerup lands on the first suggestion, which Bits picks on pointerup.
		The rounded frame clips and the chips scroll inside its padding, so a
		scrollbar runs on the straight part of the edge (as SearchBar's list):
		3px down the 12px corner curves 4px in, which the 5px on the right
		clears. The scroller keeps 4px above and below, room for a chip's
		focus ring, which a scroll box clips at its edge.
	-->
	<div
		class="overflow-hidden rounded-md border border-border bg-surface-elevated py-0.5 pr-1 pl-2 transition focus-within:border-primary"
	>
		<div
			class="flex max-h-[8.5rem] min-h-10 flex-wrap items-center gap-1.5 overflow-y-auto overscroll-contain py-1 pr-1"
		>
			{#each chips as person (person.id)}
				{@const bad = failed.has(person.id)}
				<span
					in:scale={grow}
					animate:flip={{ duration: flipDuration, easing: cubicOut }}
					class={[
						'inline-flex max-w-full items-center gap-1.5 rounded-pill py-0.5 pl-0.5 text-body-sm font-semibold',
						bad ? 'bg-destructive-soft text-destructive' : 'bg-accent text-accent-foreground'
					]}
				>
					<PersonMark {person} size="xs-plus" />
					<span class="min-w-0 truncate"
						>{person.displayName}{#if shared.has(person.displayName)}<span
								class="ml-1 font-normal opacity-75"
							>
								@{person.username}</span
							>{/if}</span
					>
					{#if bad}<span class="sr-only">{m.list_share_failed()}</span>{/if}
					<button
						type="button"
						aria-label={m.list_share_unpick({ name: person.displayName })}
						onclick={() => unpick(person.id)}
						{disabled}
						class="inline-flex size-6 shrink-0 items-center justify-center rounded-full hover-tint focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					>
						<X class="size-3.5" aria-hidden="true" />
					</button>
				</span>
			{/each}
			<Combobox.Input
				bind:ref={input}
				aria-label={m.list_share_pick()}
				placeholder={none ? m.list_share_everyone() : m.list_share_search()}
				oninput={(event) => {
					query = event.currentTarget.value;
					open = true;
				}}
				onclick={() => (open = true)}
				{onkeydown}
				class="h-8 min-w-[8rem] flex-1 bg-transparent px-1.5 text-body outline-none placeholder:text-text-muted disabled:cursor-not-allowed"
			/>
		</div>
	</div>
	<!--
		Static, in the flow under the field rather than floating over the
		dialog: inside a bottom sheet a floating list would be clipped by the
		sheet's scroll area, and pushing "Has access" down keeps everything
		reachable by scrolling. Framed like the field: the frame clips, the
		list scrolls inside it.
	-->
	<Combobox.ContentStatic
		class="mt-1.5 origin-top overflow-hidden rounded-2xl border border-popover-border bg-popover px-1.5 py-2 shadow-dialog motion-safe:animate-drop motion-reduce:animate-appear"
	>
		<div class="max-h-[17.5rem] overflow-y-auto overscroll-contain">
			{#each suggestions as person (person.id)}
				<Combobox.Item
					value={person.id}
					label={person.displayName}
					class="flex cursor-pointer items-center gap-3 rounded-sm px-2.5 py-2 text-text outline-none data-highlighted:bg-background"
				>
					<PersonMark {person} size="md" />
					<span class="flex min-w-0 flex-col">
						<span class="text-body font-semibold wrap-break-word">{person.displayName}</span>
						<span class="text-caption wrap-break-word text-text-muted">@{person.username}</span>
					</span>
				</Combobox.Item>
			{:else}
				<p class="px-2.5 py-2 text-body-sm text-text-muted">{m.list_share_nobody()}</p>
			{/each}
		</div>
	</Combobox.ContentStatic>
</Combobox.Root>
<p class="sr-only" aria-live="polite">{said}</p>
