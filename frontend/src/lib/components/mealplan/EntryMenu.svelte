<script lang="ts">
	import BookOpen from '@lucide/svelte/icons/book-open';
	import CalendarDays from '@lucide/svelte/icons/calendar-days';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import Users from '@lucide/svelte/icons/users';
	import { Dialog, DropdownMenu } from 'bits-ui';
	import { goto } from '$app/navigation';
	import type { PlanEntry } from '#lib/api/mealplan.js';
	import ServingsStepper from '#lib/components/recipe/ServingsStepper.svelte';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import DialogActions from '#lib/components/ui/DialogActions.svelte';
	import { recipeHref } from '#lib/mealplan/entries.js';
	import { getToList } from '#lib/mealplan/to-list.js';
	import { listEntryHref } from '#lib/shopping/href.js';
	import { byListName } from '#lib/shopping/plan-ref.js';
	import { m } from '#lib/paraglide/messages.js';

	let {
		entry,
		trigger,
		onmove,
		onservings,
		onremove
	}: {
		entry: PlanEntry;
		/** The ⋯'s look: round on a card's photo, plain beside a row. */
		trigger: string;
		onmove: () => void;
		onservings: (servings: number) => void;
		onremove: () => void;
	} = $props();

	const toList = getToList();
	/** Three lists at most: the card's stamps and their "+n" reach every one. */
	const lists = $derived(byListName(entry.lists).slice(0, 3));
	let servingsOpen = $state(false);
	let removing = false;
	let servings = $state(0);

	function openServings() {
		servings = entry.servings ?? 1;
		servingsOpen = true;
	}

	function save() {
		servingsOpen = false;
		if (servings !== entry.servings) onservings(servings);
	}

	// A press on the ⋯ is the menu's, never the start of a drag of its card.
	const keep = (event: Event) => event.stopPropagation();

	const item =
		'flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text transition outline-none data-highlighted:bg-popover-highlight';
	// A list's name may be long: it wraps, so its "öffnen" is never cut.
	const listItem =
		'flex min-h-10 items-center gap-2 rounded-sm px-3 py-2 text-body-sm text-text transition outline-none data-highlighted:bg-popover-highlight';
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		aria-label={m.plan_entry_menu({ title: entry.title })}
		data-entry-menu
		onmousedown={keep}
		ontouchstart={keep}
		class="inline-flex shrink-0 items-center justify-center rounded-full transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {trigger}"
	>
		<Ellipsis class="size-4" aria-hidden="true" />
	</DropdownMenu.Trigger>
	<DropdownMenu.Portal>
		<DropdownMenu.Content
			preventScroll={false}
			onCloseAutoFocus={(event) => {
				// Removed, the ⋯ is gone with its card: the week hands the focus on.
				if (removing) event.preventDefault();
				removing = false;
			}}
			sideOffset={4}
			align="end"
			collisionPadding={12}
			class="z-50 w-64 max-w-[calc(100vw-24px)] rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
		>
			{#if entry.recipeSlug}
				{@const href = recipeHref(entry)}
				<DropdownMenu.Item onSelect={() => href && goto(href)} class={item}>
					<BookOpen class="size-4" aria-hidden="true" />
					{m.plan_entry_open()}
				</DropdownMenu.Item>
			{/if}
			<DropdownMenu.Item onSelect={onmove} class={item}>
				<CalendarDays class="size-4" aria-hidden="true" />
				{m.plan_entry_move()}
			</DropdownMenu.Item>
			{#if entry.servings !== null}
				<DropdownMenu.Item onSelect={openServings} class={item}>
					<Users class="size-4" aria-hidden="true" />
					{m.plan_entry_servings()}
				</DropdownMenu.Item>
			{/if}
			{#if toList && entry.recipeId !== null && entry.servings !== null}
				<DropdownMenu.Item onSelect={() => toList(entry)} class={item}>
					<ShoppingBasket class="size-4" aria-hidden="true" />
					{entry.lists.length > 0 ? m.plan_entry_to_another_list() : m.plan_entry_to_list()}
				</DropdownMenu.Item>
			{/if}
			{#if entry.lists.length > 0}
				<DropdownMenu.Separator class="mx-1 my-1.5 h-px bg-popover-border" />
				<DropdownMenu.Group>
					<DropdownMenu.GroupHeading class="px-3 pt-1 pb-1 text-caption text-text-muted"
						>{m.plan_entry_on_lists()}</DropdownMenu.GroupHeading
					>
					{#each lists as list (list.id)}
						<!-- A long name wraps: cut, it would lose the "öffnen" that says what the item does. -->
						<DropdownMenu.Item
							onSelect={() => goto(listEntryHref(list.id, entry.id))}
							class={listItem}
						>
							<ShoppingBasket class="size-4 shrink-0 text-success-foreground" aria-hidden="true" />
							<span class="min-w-0 flex-1 break-words"
								>{m.plan_entry_open_list({ name: list.name })}</span
							>
						</DropdownMenu.Item>
					{/each}
				</DropdownMenu.Group>
			{/if}
			<DropdownMenu.Separator class="mx-1 my-1.5 h-px bg-popover-border" />
			<DropdownMenu.Item
				onSelect={() => {
					removing = true;
					onremove();
				}}
				class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-destructive transition outline-none data-highlighted:bg-destructive-soft"
			>
				<Trash2 class="size-4" aria-hidden="true" />
				{m.plan_entry_remove()}
			</DropdownMenu.Item>
		</DropdownMenu.Content>
	</DropdownMenu.Portal>
</DropdownMenu.Root>

<BaseDialog bind:open={servingsOpen}>
	<Dialog.Title class="font-display text-heading font-medium"
		>{m.plan_entry_servings()}</Dialog.Title
	>
	<Dialog.Description class="mt-1 text-body text-text-muted">{entry.title}</Dialog.Description>
	<div class="mt-5 flex justify-center">
		<ServingsStepper value={servings} onchange={(next) => (servings = next)} />
	</div>
	<DialogActions class="mt-6">
		<Button variant="ghost" onclick={() => (servingsOpen = false)}>{m.common_cancel()}</Button>
		<Button onclick={save}>{m.common_save()}</Button>
	</DialogActions>
</BaseDialog>
