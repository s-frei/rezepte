<script lang="ts">
	import { tick } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Send from '@lucide/svelte/icons/send';
	import { addComment, deleteComment, editComment, type Comment } from '$lib/api/comments';
	import Button from '$lib/components/ui/Button.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import { groupByDay, sendable } from '$lib/recipe/diary';
	import { m } from '$lib/paraglide/messages';
	import DiaryEntry from './DiaryEntry.svelte';
	import DiaryField from './DiaryField.svelte';

	let {
		recipeId,
		entries = $bindable()
	}: {
		recipeId: string;
		/** Owned by the page, whose diary tab counts them. */
		entries: Comment[];
	} = $props();

	// The composer below the entries writes new ones only; an entry is
	// edited in place, in its own bubble (DiaryEntry).
	let draft = $state('');
	let busy = $state(false);
	let deleting = $state<Comment | null>(null);
	let confirmOpen = $state(false);
	let composer = $state<HTMLTextAreaElement>();
	// Read out after a send or a delete, which otherwise happen silently.
	let announcement = $state('');

	const days = $derived(groupByDay(entries, new Date()));
	const blocked = $derived(busy || !sendable(draft));
	// The Enter hint is only true with a mouse and keyboard (DiaryField);
	// only there does the field point screen readers at it.
	const hintId = $props.id();
	const fine = matchMedia('(pointer: fine)').matches;

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (blocked) return;
		busy = true;
		const sent = draft;
		try {
			entries = [...entries, await addComment(recipeId, sent)];
			// Whatever was typed while it went out stays.
			if (draft === sent) draft = '';
			announcement = m.diary_added();
		} catch {
			toast.error(m.diary_error_add());
		} finally {
			busy = false;
		}
		// The disabled button dropped the focus; a phone would close its keyboard.
		await tick();
		composer?.focus();
	}

	/** Bound to one entry's id by its bubble, so a save cannot land elsewhere. */
	async function save(id: number, body: string) {
		try {
			const saved = await editComment(id, body);
			entries = entries.map((e) => (e.id === saved.id ? { ...saved, new: e.new } : e));
		} catch (error) {
			toast.error(m.diary_error_edit());
			throw error;
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
			{#each day.entries as entry (entry.id)}
				<DiaryEntry
					{entry}
					onsave={(body) => save(entry.id, body)}
					ondelete={() => {
						deleting = entry;
						confirmOpen = true;
					}}
				/>
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
			<DiaryField
				bind:value={draft}
				bind:element={composer}
				label={m.diary_placeholder()}
				describedby={fine ? hintId : undefined}
				plain
			/>
			<!-- A round icon on a phone, the icon and its word from md. -->
			<Button type="submit" size="sm" disabled={blocked} class="shrink-0 max-md:size-9 max-md:px-0">
				<Send class="size-[18px]" aria-hidden="true" />
				<span class="max-md:sr-only">{m.diary_add()}</span>
			</Button>
		</div>
		<!-- Only true with a keyboard and mouse: on a touch screen Enter breaks the line. -->
		<p id={hintId} class="mt-2 ml-4 hidden text-micro text-text-muted md:ml-4.5 pointer-fine:block">
			{m.diary_hint()}
		</p>
	</form>
</div>

<ConfirmDialog
	bind:open={confirmOpen}
	title={m.diary_delete_title()}
	text={m.diary_delete_text()}
	confirmLabel={m.common_delete()}
	destructive
	onconfirm={confirmDelete}
/>
