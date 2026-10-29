<script lang="ts">
	import { tick } from 'svelte';
	import { DropdownMenu } from 'bits-ui';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import { toast } from 'svelte-sonner';
	import { ApiError } from '$lib/api/client';
	import { commentImageUrl, type Comment } from '$lib/api/comments';
	import Button from '$lib/components/ui/Button.svelte';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';
	import { uploadErrorMessage } from '$lib/images/pick';
	import { entryTime, sendable } from '$lib/recipe/diary';
	import { stagePhotos, unstage, type PendingPhoto } from '$lib/recipe/photos';
	import { userColorClasses } from '$lib/user/color';
	import { m } from '$lib/paraglide/messages';
	import DiaryField from './DiaryField.svelte';
	import DiaryPhotos from './DiaryPhotos.svelte';

	let {
		entry,
		recipeId,
		onsave,
		ondelete,
		onopenphoto
	}: {
		/** `canEdit` means the signed-in member wrote it: right-aligned, in their own color. */
		entry: Comment;
		recipeId: string;
		/** Saves a new body and the full set of photos; rejects when the save failed. */
		onsave: (body: string, imageIds: string[]) => Promise<void>;
		ondelete: () => void;
		/** Opens the lightbox on this entry's photo `index`. */
		onopenphoto: (index: number) => void;
	} = $props();

	const mine = $derived(entry.canEdit);
	const paper = $derived(
		mine && entry.author ? userColorClasses(entry.author.color) : 'bg-surface-elevated'
	);

	// Editing swaps this bubble for the composer, prefilled, in place.
	let editing = $state(false);
	let draft = $state('');
	let photos = $state<PendingPhoto[]>([]);
	let busy = $state(false);
	let progress = $state<{ n: number; count: number } | null>(null);
	let field = $state<HTMLTextAreaElement>();
	let trigger = $state<HTMLButtonElement | null>(null);
	// Set when "Edit" closes the menu, so the menu does not hand focus
	// back to its trigger - the trigger is about to go, the field takes it.
	let editChosen = false;

	function startEdit() {
		editChosen = true;
		draft = entry.body;
		photos = entry.images.map((image) => ({
			key: image.id,
			preview: commentImageUrl(recipeId, image.id, 'thumb'),
			staged: image
		}));
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
			const ids = await stagePhotos(recipeId, photos, (n, count) => (progress = { n, count }));
			progress = null;
			await onsave(draft, ids);
			await stopEdit();
		} catch (error) {
			// A failed save has been toasted by the caller; the draft stays for another try.
			if (progress) toast.error(uploadErrorMessage(error));
			// 422: a picked photo's staged copy is gone; the next Save uploads them again.
			else if (error instanceof ApiError && error.status === 422) unstage(photos);
		} finally {
			busy = false;
			progress = null;
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
				readonly={busy}
				oncancel={busy ? undefined : stopEdit}
				class="basis-full"
			/>
			<!-- Cancel drops the picked files; after a failed Save some may be up
			     already, and the sweep takes those. While a save runs it waits. -->
			<DiaryPhotos bind:photos {busy} />
			<div class="ml-auto flex gap-2">
				<Button variant="ghost" disabled={busy} onclick={stopEdit}>{m.common_cancel()}</Button>
				<Button type="submit" disabled={busy || !sendable(draft)} class="whitespace-nowrap">
					{progress
						? m.diary_sending_photo({ n: progress.n, count: progress.count })
						: m.common_save()}
				</Button>
			</div>
		</form>
	{:else}
		<div
			class="max-w-[82%] min-w-0 rounded-2xl diary-paper px-4 pt-1.5 pb-2 shadow-card {paper} {entry
				.images.length
				? entry.images.length === 1
					? 'w-full md:w-auto'
					: 'w-full'
				: ''}"
		>
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
			{#if entry.images.length}
				{@const count = entry.images.length}
				<!-- One photo fills the bubble on phones and is capped at 24rem from md up, where the bubble grows with its text; two to four share one row, even at 320px. -->
				<ul
					aria-label={m.diary_photos()}
					class="mt-1.5 mb-0.5 {count > 1 ? 'grid grid-cols-4 gap-1' : 'max-w-full md:w-96'}"
				>
					{#each entry.images as image, index (image.id)}
						<li>
							<button
								type="button"
								onclick={() => onopenphoto(index)}
								aria-label={m.images_open({ number: index + 1, count })}
								class="block w-full overflow-hidden rounded-md bg-skeleton focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {count >
								1
									? 'aspect-square'
									: 'aspect-[4/3]'}"
							>
								<img
									src={commentImageUrl(recipeId, image.id, 'thumb')}
									alt=""
									loading="lazy"
									decoding="async"
									class="size-full object-cover"
								/>
							</button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</li>
