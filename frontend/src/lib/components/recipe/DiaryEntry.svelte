<script lang="ts">
	import { tick } from 'svelte';
	import { DropdownMenu } from 'bits-ui';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import type { Comment } from '$lib/api/comments';
	import Button from '$lib/components/ui/Button.svelte';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';
	import { entryTime, sendable } from '$lib/recipe/diary';
	import { userColorClasses } from '$lib/user/color';
	import { m } from '$lib/paraglide/messages';
	import DiaryField from './DiaryField.svelte';

	let {
		entry,
		onsave,
		ondelete
	}: {
		/** `canEdit` means the signed-in member wrote it: right-aligned, in their own color. */
		entry: Comment;
		/** Saves a new body for this entry; rejects when the save failed. */
		onsave: (body: string) => Promise<void>;
		ondelete: () => void;
	} = $props();

	const mine = $derived(entry.canEdit);
	const paper = $derived(
		mine && entry.author ? userColorClasses(entry.author.color) : 'bg-surface-elevated'
	);

	// Editing swaps this bubble for the composer, prefilled, in place.
	let editing = $state(false);
	let draft = $state('');
	let busy = $state(false);
	let field = $state<HTMLTextAreaElement>();
	let trigger = $state<HTMLButtonElement | null>(null);
	// Set when "Edit" closes the menu, so the menu does not hand focus
	// back to its trigger - the trigger is about to go, the field takes it.
	let editChosen = false;

	function startEdit() {
		editChosen = true;
		draft = entry.body;
		editing = true;
	}

	async function closeMenu(event: Event) {
		if (!editChosen) return;
		editChosen = false;
		event.preventDefault();
		await tick();
		field?.focus();
	}

	async function stopEdit() {
		editing = false;
		await tick();
		trigger?.focus();
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (busy || !sendable(draft)) return;
		busy = true;
		try {
			await onsave(draft);
			await stopEdit();
		} catch {
			// The caller has said so; the draft stays for another try.
		} finally {
			busy = false;
		}
	}
</script>

<li class="flex items-end gap-2.5 {mine ? 'flex-row-reverse' : ''}">
	{#if entry.author}
		<PersonMark person={entry.author} size="sm" />
	{:else}
		<!-- A former member: an empty circle drawn in the leader's dots. -->
		<span
			aria-hidden="true"
			class="size-7 shrink-0 rounded-full border-2 border-dotted border-handle"
		></span>
	{/if}
	{#if editing}
		<form class="flex min-w-0 flex-1 flex-wrap items-end gap-2" onsubmit={save}>
			<DiaryField
				bind:value={draft}
				bind:element={field}
				label={m.diary_edit_label()}
				oncancel={stopEdit}
				class="basis-full"
			/>
			<div class="ml-auto flex gap-2">
				<Button variant="ghost" onclick={stopEdit}>{m.common_cancel()}</Button>
				<Button type="submit" disabled={busy || !sendable(draft)}>
					{m.common_save()}
				</Button>
			</div>
		</form>
	{:else}
		<div class="max-w-[82%] min-w-0 rounded-2xl diary-paper px-4 pt-1.5 pb-2 shadow-card {paper}">
			<div
				class="flex flex-wrap items-center gap-x-1.5 text-micro leading-[26px] {mine
					? ''
					: 'text-text-muted'}"
			>
				{#if !mine}
					<span class="font-semibold text-text"
						>{entry.author?.displayName ?? m.diary_former_member()}</span
					>
					<span aria-hidden="true">·</span>
				{/if}
				<time datetime={entry.createdAt}>{entryTime(entry.createdAt)}</time>
				{#if entry.editedAt}<span>· {m.diary_edited()}</span>{/if}
				{#if entry.new}
					<span class="size-[7px] rounded-full bg-primary" aria-hidden="true"></span>
					<span class="sr-only">{m.diary_new()}</span>
				{/if}
				{#if entry.canEdit || entry.canDelete}
					<DropdownMenu.Root>
						<DropdownMenu.Trigger
							bind:ref={trigger}
							aria-label={m.diary_entry_menu()}
							class="-my-1 -mr-2 ml-auto rounded-full p-1 focus-visible:outline-2 focus-visible:outline-primary"
						>
							<Ellipsis class="size-4" aria-hidden="true" />
						</DropdownMenu.Trigger>
						<DropdownMenu.Portal>
							<DropdownMenu.Content
								preventScroll={false}
								sideOffset={4}
								align="end"
								onCloseAutoFocus={closeMenu}
								class="z-50 w-44 rounded-2xl border border-popover-border bg-popover p-2 shadow-dialog"
							>
								{#if entry.canEdit}
									<DropdownMenu.Item
										onSelect={startEdit}
										class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-text outline-none data-highlighted:bg-background"
									>
										{m.diary_edit()}
									</DropdownMenu.Item>
								{/if}
								{#if entry.canDelete}
									<DropdownMenu.Item
										onSelect={ondelete}
										class="flex h-10 items-center gap-2 rounded-sm px-3 text-body-sm text-destructive outline-none data-highlighted:bg-destructive-soft"
									>
										{m.common_delete()}
									</DropdownMenu.Item>
								{/if}
							</DropdownMenu.Content>
						</DropdownMenu.Portal>
					</DropdownMenu.Root>
				{/if}
			</div>
			<!-- 26px lines: the rule spacing of `diary-paper`. -->
			<p
				class="font-display text-card-sm leading-[26px] wrap-anywhere hyphens-auto whitespace-pre-wrap italic"
			>
				{entry.body}
			</p>
		</div>
	{/if}
</li>
