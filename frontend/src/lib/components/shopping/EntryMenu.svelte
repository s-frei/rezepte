<script lang="ts">
	import BookOpen from '@lucide/svelte/icons/book-open';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import Users from '@lucide/svelte/icons/users';
	import { Dialog, DropdownMenu } from 'bits-ui';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { ShoppingEntry } from '#lib/api/shopping.js';
	import ServingsStepper from '#lib/components/recipe/ServingsStepper.svelte';
	import BaseDialog from '#lib/components/ui/BaseDialog.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let {
		entry,
		onservings,
		onremove
	}: {
		entry: ShoppingEntry;
		onservings: (servings: number) => void;
		onremove: () => void;
	} = $props();

	let servingsOpen = $state(false);
	let servings = $state(0);

	function openServings() {
		servings = entry.servings;
		servingsOpen = true;
	}

	function save() {
		servingsOpen = false;
		if (servings !== entry.servings) onservings(servings);
	}

	const item =
		'flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text transition hover:bg-background';
</script>

<DropdownMenu.Root>
	<!-- Round and outlined like the list header's ⋯, so it reads as a button where "remove" lives. -->
	<DropdownMenu.Trigger
		aria-label={m.list_entry_menu({ title: entry.title })}
		class="inline-flex size-10 shrink-0 items-center justify-center rounded-full border border-border bg-surface hover-tint text-text transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]"
	>
		<Ellipsis class="size-5" aria-hidden="true" />
	</DropdownMenu.Trigger>
	<DropdownMenu.Portal>
		<DropdownMenu.Content
			preventScroll={false}
			sideOffset={4}
			align="end"
			class="z-50 w-56 rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
		>
			{#if entry.recipeSlug}
				{@const slug = entry.recipeSlug}
				<DropdownMenu.Item onSelect={() => goto(resolve('/recipes/[slug]', { slug }))} class={item}>
					<BookOpen class="size-4" aria-hidden="true" />
					{m.list_entry_open()}
				</DropdownMenu.Item>
			{/if}
			<DropdownMenu.Item onSelect={openServings} class={item}>
				<Users class="size-4" aria-hidden="true" />
				{m.list_entry_servings()}
			</DropdownMenu.Item>
			<DropdownMenu.Separator class="mx-1 my-1.5 h-px bg-popover-border" />
			<DropdownMenu.Item
				onSelect={onremove}
				class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-destructive transition hover:bg-destructive-soft"
			>
				<Trash2 class="size-4" aria-hidden="true" />
				{m.list_entry_remove()}
			</DropdownMenu.Item>
		</DropdownMenu.Content>
	</DropdownMenu.Portal>
</DropdownMenu.Root>

<BaseDialog bind:open={servingsOpen}>
	<Dialog.Title class="font-display text-heading font-medium"
		>{m.list_entry_servings()}</Dialog.Title
	>
	<Dialog.Description class="mt-1 text-body text-text-muted">{entry.title}</Dialog.Description>
	<div class="mt-5 flex justify-center">
		<ServingsStepper value={servings} onchange={(next) => (servings = next)} />
	</div>
	<div class="mt-6 flex justify-end gap-2.5">
		<Button variant="ghost" onclick={() => (servingsOpen = false)}>{m.common_cancel()}</Button>
		<Button onclick={save}>{m.common_save()}</Button>
	</div>
</BaseDialog>
