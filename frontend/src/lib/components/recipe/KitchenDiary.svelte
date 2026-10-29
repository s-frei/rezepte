<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { toast } from 'svelte-sonner';
	import Check from '@lucide/svelte/icons/check';
	import ImagePlus from '@lucide/svelte/icons/image-plus';
	import Send from '@lucide/svelte/icons/send';
	import { ApiError } from '#lib/api/client.js';
	import {
		addComment,
		addCommentImageToRecipe,
		commentImageUrl,
		deleteComment,
		editComment,
		listComments,
		type Comment
	} from '#lib/api/comments.js';
	import { MAX_IMAGES } from '#lib/api/recipes.js';
	import Button from '#lib/components/ui/Button.svelte';
	import ConfirmDialog from '#lib/components/ui/ConfirmDialog.svelte';
	import Lightbox from '#lib/components/ui/Lightbox.svelte';
	import { uploadErrorMessage } from '#lib/images/pick.js';
	import { refusalOr } from '#lib/recipe/access.js';
	import { groupByDay, sendable } from '#lib/recipe/diary.js';
	import { revokePreviews, stagePhotos, unstage, type PendingPhoto } from '#lib/recipe/photos.js';
	import { m } from '#lib/paraglide/messages.js';
	import DiaryEntry from './DiaryEntry.svelte';
	import DiaryField from './DiaryField.svelte';
	import DiaryPhotos from './DiaryPhotos.svelte';

	let {
		recipeId,
		entries = $bindable(),
		title,
		canEdit,
		onrecipephoto
	}: {
		recipeId: string;
		/** Owned by the page, whose diary tab counts them. */
		entries: Comment[];
		/** The recipe's, for the lightbox. */
		title: string;
		/** The reader may edit the recipe, so may add a comment photo to its photos. */
		canEdit: boolean;
		/** A comment photo became a recipe photo: the page reloads the recipe. */
		onrecipephoto?: () => void;
	} = $props();

	// The composer below the entries writes new ones only; an entry is
	// edited in place, in its own bubble (DiaryEntry).
	let draft = $state('');
	let photos = $state<PendingPhoto[]>([]);
	let busy = $state(false);
	// The upload running during a send, for the Send button's label.
	let progress = $state<{ n: number; count: number } | null>(null);
	let deleting = $state<Comment | null>(null);
	let confirmOpen = $state(false);
	let composer = $state<HTMLTextAreaElement>();
	// Read out after a send or a delete, which otherwise happen silently.
	let announcement = $state('');
	// The entry this visit just sent rises into place; nothing else does, so
	// the list never moves on load or when a reload brings someone's entry.
	let sentId = $state<number | null>(null);
	// A deleted entry fades while its row closes, so the entries after it
	// and the composer below move up as one. Not `animate:flip`: that lifts
	// the leaving entry out of the flow, the composer jumped up at once and
	// the next entry slid down over it. The list's gap closes with the row -
	// the one above it when it is the last - or it would snap at the end.
	const leaving = $derived(prefersReducedMotion.current ? 0 : 200);
	function leave(node: HTMLElement) {
		const height = node.offsetHeight;
		const gap = parseFloat(getComputedStyle(node.parentElement!).rowGap) || 0;
		const side = node.nextElementSibling ? 'margin-bottom' : 'margin-top';
		return {
			duration: leaving,
			easing: cubicOut,
			css: (t: number) =>
				`overflow: hidden; opacity: ${t}; height: ${t * height}px; ${side}: ${(t - 1) * gap}px`
		};
	}

	const days = $derived(groupByDay(entries, new Date()));
	const blocked = $derived(busy || !sendable(draft));
	// The Enter hint is only true with a mouse and keyboard (DiaryField);
	// only there does the field point screen readers at it.
	const hintId = $props.id();
	const fine = matchMedia('(pointer: fine)').matches;

	// Switching recipes mid-send drops this diary; the answer must not land in the next one.
	let destroyed = false;
	onDestroy(() => (destroyed = true));

	/**
	 * After a 422 a comment may have landed all the same (an earlier try whose
	 * answer was lost): the list from the server, keeping this visit's New marks.
	 */
	async function reload() {
		try {
			const fresh = await listComments(recipeId);
			if (destroyed) return;
			const known = new Map(entries.map((e) => [e.id, e.new]));
			entries = fresh.map((c) => ({ ...c, new: known.get(c.id) ?? c.new }));
		} catch {
			// The list stays as it was.
		}
	}

	function rejected(error: unknown) {
		return error instanceof ApiError && error.status === 422;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (blocked) return;
		busy = true;
		const sent = draft;
		try {
			// Photos go up only now; on a failure the text and the photos stay,
			// and those already up are not sent twice (stagePhotos).
			const ids = await stagePhotos(recipeId, photos, (n, count) => {
				progress = { n, count };
				announcement = m.diary_sending_photo({ n, count });
			});
			progress = null;
			const added = await addComment(recipeId, sent, ids);
			if (destroyed) return;
			sentId = added.id;
			if (!entries.some((e) => e.id === added.id)) entries = [...entries, added];
			// Whatever was typed while it went out stays.
			if (draft === sent) draft = '';
			revokePreviews(photos);
			photos = [];
			announcement = m.diary_added();
		} catch (error) {
			if (progress) {
				// The upload's own 422 (a photo too small, say) stays on its tile.
				toast.error(uploadErrorMessage(error));
			} else if (rejected(error)) {
				// The comment's 422: a photo id is gone (evicted or swept); the next Send uploads them again.
				toast.error(m.diary_error_add());
				unstage(photos);
				await reload();
			} else {
				toast.error(m.diary_error_add());
			}
		} finally {
			busy = false;
			progress = null;
		}
		// The disabled button dropped the focus; a phone would close its keyboard.
		await tick();
		composer?.focus();
	}

	/** Bound to one entry's id by its bubble, so a save cannot land elsewhere. */
	async function save(id: number, body: string, imageIds: string[]) {
		try {
			const saved = await editComment(id, body, imageIds);
			if (destroyed) return;
			entries = entries.map((e) => (e.id === saved.id ? { ...saved, new: e.new } : e));
		} catch (error) {
			toast.error(m.diary_error_edit());
			// The bubble unstages its photos (DiaryEntry).
			if (rejected(error)) await reload();
			throw error;
		}
	}

	// One lightbox for every comment's photos, fed with the clicked comment's.
	let viewingId = $state<number | null>(null);
	const viewing = $derived(entries.find((e) => e.id === viewingId));
	let lightboxOpen = $state(false);
	let lightboxIndex = $state(0);
	let adding = $state(false);
	const noteId = `${hintId}-note`;

	function openPhoto(entry: Comment, index: number) {
		viewingId = entry.id;
		lightboxIndex = index;
		lightboxOpen = true;
	}

	function markInRecipe(commentId: number, imageId: string) {
		entries = entries.map((e) =>
			e.id === commentId
				? {
						...e,
						images: e.images.map((i) => (i.id === imageId ? { ...i, inRecipe: true } : i))
					}
				: e
		);
	}

	async function toRecipe(event: MouseEvent, index: number) {
		// Once in the recipe the button turns disabled and drops the focus: Close takes it.
		const close = (event.currentTarget as HTMLElement)
			.closest('[role="dialog"]')
			?.querySelector<HTMLElement>('[data-dialog-close]');
		const entry = viewing;
		const image = entry?.images[index];
		if (!entry || !image || adding) return;
		adding = true;
		try {
			await addCommentImageToRecipe(recipeId, image.id);
			markInRecipe(entry.id, image.id);
			close?.focus();
			toast.success(m.diary_photo_to_recipe_done());
			onrecipephoto?.();
		} catch (error) {
			// 409: someone else was quicker; the photo is there either way.
			if (error instanceof ApiError && error.status === 409) {
				markInRecipe(entry.id, image.id);
				close?.focus();
			} else if (error instanceof ApiError && error.status === 422) {
				toast.error(m.images_limit_reached({ count: MAX_IMAGES }));
			} else {
				toast.error(refusalOr(error, m.diary_photo_to_recipe_error()));
			}
		} finally {
			adding = false;
		}
	}

	async function confirmDelete() {
		const target = deleting;
		if (!target) return;
		try {
			await deleteComment(target.id);
			entries = entries.filter((e) => e.id !== target.id);
			announcement = m.diary_deleted();
			// The removed entry's menu held the focus.
			await tick();
			composer?.focus();
		} catch {
			toast.error(m.diary_error_delete());
		}
	}
</script>

<!-- The tab panel around this is the region, named by its tab. -->
<div>
	{#each days as day (day.key)}
		<h3 class="my-4 flex items-center gap-2.5 font-display text-body-sm text-text-muted italic">
			<span class="flex-1 border-b-2 border-dotted border-border" aria-hidden="true"></span>
			{day.label}
			<span class="flex-1 border-b-2 border-dotted border-border" aria-hidden="true"></span>
		</h3>
		<ul class="flex flex-col gap-3">
			<!-- The rise is CSS, not `in:`: the first entry of a day arrives with
			     its day, and a transition would not play for a new day's list.
			     A delete that empties a day takes the day with it at once. -->
			{#each day.entries as entry (entry.id)}
				<li
					class={entry.id === sentId ? 'motion-safe:animate-rise' : ''}
					onanimationend={(event) => {
						if (event.target === event.currentTarget) sentId = null;
					}}
					out:leave
				>
					<DiaryEntry
						{entry}
						{recipeId}
						onsave={(body, imageIds) => save(entry.id, body, imageIds)}
						onopenphoto={(index) => openPhoto(entry, index)}
						ondelete={() => {
							deleting = entry;
							confirmOpen = true;
						}}
					/>
				</li>
			{/each}
		</ul>
	{:else}
		<p class="font-display text-body-sm text-text-muted italic">{m.diary_count_none()}</p>
	{/each}
	<p class="sr-only" aria-live="polite">{announcement}</p>

	<!-- One notepad: the field and its send button share the box, which
	     carries the field's focus ring. -->
	<form class="mt-5" onsubmit={submit}>
		<div
			class="flex flex-wrap items-end gap-2 rounded-3xl border border-border bg-surface-elevated py-1.5 pr-1.5 pl-4 shadow-card has-[textarea:focus-visible]:outline-2 has-[textarea:focus-visible]:outline-primary md:py-2 md:pr-2 md:pl-4.5"
		>
			<!-- The tiles take a row of their own above; the picker sits before the field. -->
			<DiaryPhotos bind:photos {busy} />
			<DiaryField
				bind:value={draft}
				bind:element={composer}
				readonly={busy}
				label={photos.length ? m.diary_placeholder_photo() : m.diary_placeholder()}
				describedby={fine ? hintId : undefined}
				plain
			/>
			<!-- A round icon on a phone, the icon and its word from md; the
			     upload count shows on every screen while photos go up. -->
			<Button
				type="submit"
				size="sm"
				disabled={blocked}
				class="shrink-0 whitespace-nowrap {progress ? '' : 'max-md:size-9 max-md:px-0'}"
			>
				<Send class="size-[18px]" aria-hidden="true" />
				<span class={progress ? '' : 'max-md:sr-only'}>
					{progress
						? m.diary_sending_photo({ n: progress.n, count: progress.count })
						: m.diary_add()}
				</span>
			</Button>
		</div>
		<!-- Only true with a keyboard and mouse: on a touch screen Enter breaks the line. -->
		<p id={hintId} class="mt-2 ml-4 hidden text-micro text-text-muted md:ml-4.5 pointer-fine:block">
			{m.diary_hint()}
		</p>
	</form>
</div>

{#snippet toRecipeAction(index: number)}
	{@const image = viewing?.images[index]}
	{#if canEdit && image}
		<div class="flex basis-full flex-wrap items-center gap-x-3 gap-y-1">
			<button
				type="button"
				disabled={image.inRecipe}
				aria-disabled={adding}
				aria-describedby={image.inRecipe ? undefined : noteId}
				onclick={(event) => toRecipe(event, index)}
				class="inline-flex h-9 items-center gap-2 rounded-pill bg-lightbox-foreground/15 px-3.5 text-body-sm font-semibold whitespace-nowrap text-lightbox-foreground transition hover:bg-lightbox-foreground/25 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:bg-transparent disabled:px-0 disabled:font-medium disabled:opacity-70 aria-disabled:opacity-70"
			>
				{#if image.inRecipe}
					<Check class="size-4" aria-hidden="true" />
					{m.diary_photo_in_recipe()}
				{:else}
					<ImagePlus class="size-4" aria-hidden="true" />
					{m.diary_photo_to_recipe()}
				{/if}
			</button>
			{#if !image.inRecipe}
				<p id={noteId} class="text-caption opacity-70">{m.diary_photo_to_recipe_note()}</p>
			{/if}
		</div>
	{/if}
{/snippet}

<Lightbox
	bind:open={lightboxOpen}
	bind:index={lightboxIndex}
	{recipeId}
	{title}
	images={viewing?.images ?? []}
	src={(id, variant) => commentImageUrl(recipeId, id, variant)}
	action={toRecipeAction}
/>

<ConfirmDialog
	bind:open={confirmOpen}
	title={m.diary_delete_title()}
	text={m.diary_delete_text()}
	confirmLabel={m.common_delete()}
	destructive
	onconfirm={confirmDelete}
/>
