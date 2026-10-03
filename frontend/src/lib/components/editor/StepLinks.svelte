<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import Link2 from '@lucide/svelte/icons/link-2';
	import Timer from '@lucide/svelte/icons/timer';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	import X from '@lucide/svelte/icons/x';
	import type { StepTime } from '$lib/api/recipes';
	import { m } from '$lib/paraglide/messages';
	import type { FormRef } from '$lib/recipe/form';
	import { refLabel, refView, type PickerEntry } from '$lib/recipe/step-references';
	import { ACTION, ROW } from './link-list';
	import { formatDuration } from '$lib/recipe/times';

	/**
	 * What a step's links and times amount to, below the step: one quiet line that says
	 * how many there are and opens into the list, the link button beside it
	 * while the step is being edited, and - never folded away - every link that
	 * points nowhere any more.
	 *
	 * The underline in the sentence is where a link is seen; this is where it
	 * is counted, reached from the keyboard and taken off. Listing every link
	 * in full under every step said everything twice and made the list longer
	 * than the step it describes.
	 */
	let {
		stepId,
		links,
		pending,
		times,
		pendingTimes,
		entries,
		editing,
		onlinkword,
		onmarktime,
		onunlink,
		onaccept,
		ondismiss,
		onaccepttime,
		onremovetime
	}: {
		stepId: string;
		/** The step's links whose word is in the text, in the order they appear there. */
		links: FormRef[];
		/** The matcher's open proposals for the step, in the same order. */
		pending: FormRef[];
		/** The step's stored times whose phrase is in the text, in text order. */
		times: StepTime[];
		/** The detector's open time proposals, in the same order. */
		pendingTimes: StepTime[];
		entries: PickerEntry[];
		/** Whether the step has focus; the link and time buttons are only offered then. */
		editing: boolean;
		onlinkword: () => void;
		onmarktime: () => void;
		onunlink: (word: string) => void;
		onaccept: (ref: FormRef) => void;
		/** Turns down a link proposal or a time proposal, by its word or phrase. */
		ondismiss: (word: string) => void;
		onaccepttime: (time: StepTime) => void;
		onremovetime: (phrase: string) => void;
	} = $props();

	let open = $state(false);

	const broken = $derived(links.filter((ref) => !refView(ref, entries).resolved));
	const working = $derived(links.filter((ref) => refView(ref, entries).resolved));
	const suggested = $derived(pending.length + pendingTimes.length);
	const listed = $derived(working.length + times.length + suggested);

	const summary = $derived(
		[
			working.length > 0 ? m.editor_reference_summary_links({ count: working.length }) : '',
			times.length === 1
				? m.editor_time_summary_one()
				: times.length > 1
					? m.editor_time_summary({ count: times.length })
					: '',
			suggested === 1
				? m.editor_reference_summary_suggestions_one()
				: suggested > 1
					? m.editor_reference_summary_suggestions({ count: suggested })
					: ''
		]
			.filter((part) => part !== '')
			.join(', ')
	);
</script>

{#if broken.length > 0}
	<!--
		A link whose ingredient is gone needs a decision before the recipe can
		be saved, so it is never folded into the count.
	-->
	<ul class="mt-1.5 list-none space-y-1">
		{#each broken as ref (ref.word)}
			{@const view = refView(ref, entries)}
			<li class="flex items-center gap-1.5 text-caption text-destructive">
				<TriangleAlert class="size-3.5 shrink-0" aria-hidden="true" />
				<span class="min-w-0">
					<span class="font-medium">{m.editor_reference_unresolved()}:</span>
					{view.name}{#if view.group}&nbsp;({view.group}){/if}
				</span>
				<button
					type="button"
					onclick={() => onunlink(ref.word)}
					aria-label={m.editor_reference_remove({ label: refLabel(ref, entries) })}
					class="-my-1.5 ml-auto flex size-8 shrink-0 items-center justify-center rounded-pill transition hover:text-text focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
				>
					<X class="size-3.5" aria-hidden="true" />
				</button>
			</li>
		{/each}
	</ul>
{/if}

{#if editing || listed > 0}
	<div class="mt-1 flex min-h-8 flex-wrap items-center gap-x-2">
		{#if editing}
			<!--
				`mousedown` is prevented so the click never takes focus out of
				the step: the selection the button acts on is the one in the
				editor, and blurring the field first would lose sight of it.
			-->
			<button
				type="button"
				onmousedown={(event) => event.preventDefault()}
				onclick={onlinkword}
				class="-ml-1 flex items-center gap-1 rounded-pill px-1 py-1 text-caption text-text-muted transition hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				<Link2 class="size-3.5" aria-hidden="true" />
				{m.editor_reference_link_word()}
			</button>
			<button
				type="button"
				onmousedown={(event) => event.preventDefault()}
				onclick={onmarktime}
				class="flex items-center gap-1 rounded-pill px-1 py-1 text-caption text-text-muted transition hover:text-time-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				<Timer class="size-3.5" aria-hidden="true" />
				{m.editor_time_mark()}
			</button>
		{/if}
		{#if listed > 0}
			<!--
				`mousedown` is prevented here too: a tap that focused the toggle
				would start editing, the buttons above would push it onto the
				next line of a phone, and the click would land beside it.
			-->
			<button
				type="button"
				aria-expanded={open}
				aria-controls="step-links-{stepId}"
				onmousedown={(event) => event.preventDefault()}
				onclick={() => (open = !open)}
				class="-mr-1 ml-auto flex items-center gap-1 rounded-pill px-1 py-1 text-caption transition hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {suggested >
				0
					? 'text-primary'
					: 'text-text-muted'}"
			>
				{summary}
				<ChevronDown
					class="size-3.5 transition-transform motion-reduce:transition-none {open
						? 'rotate-180'
						: ''}"
					aria-hidden="true"
				/>
			</button>
		{/if}
	</div>
{/if}

{#if open && listed > 0}
	<ul
		id="step-links-{stepId}"
		aria-label={m.editor_reference_links()}
		class="mb-1 list-none divide-y divide-dashed divide-border border-t border-dashed border-border"
	>
		{#each working as ref (ref.word)}
			{@const view = refView(ref, entries)}
			<li class={ROW}>
				<span class="min-w-0">
					<span class="font-medium">{view.name}</span>
					{#if view.group}<span class="text-caption text-text-muted">&nbsp;{view.group}</span>{/if}
				</span>
				<span class="text-caption text-text-muted tabular-nums">{view.amount}</span>
				<button
					type="button"
					onclick={() => onunlink(ref.word)}
					aria-label={m.editor_reference_remove({ label: refLabel(ref, entries) })}
					class="{ACTION} hover:text-destructive"
				>
					<X class="size-3.5" aria-hidden="true" />
				</button>
			</li>
		{/each}
		{#each pending as ref (ref.word)}
			{@const view = refView(ref, entries)}
			{@const label = refLabel(ref, entries)}
			<!-- A proposal reads in the color of its dotted underline. -->
			<li class={ROW}>
				<span class="min-w-0 text-primary">
					<span class="font-medium">{view.name}</span>
					{#if view.group}<span class="text-caption">&nbsp;{view.group}</span>{/if}
				</span>
				<span class="text-caption text-primary tabular-nums">{view.amount}</span>
				<span class="flex">
					<button
						type="button"
						onclick={() => onaccept(ref)}
						aria-label={m.editor_reference_accept({ label })}
						class="{ACTION} hover:text-primary"
					>
						<Check class="size-3.5" aria-hidden="true" />
					</button>
					<button
						type="button"
						onclick={() => ondismiss(ref.word)}
						aria-label={m.editor_reference_dismiss({ label })}
						class="{ACTION} hover:text-destructive"
					>
						<X class="size-3.5" aria-hidden="true" />
					</button>
				</span>
			</li>
		{/each}
		{#each times as time (time.phrase)}
			<li class={ROW}>
				<span class="flex min-w-0 items-center gap-1.5">
					<Timer class="size-3.5 shrink-0 text-time-foreground" aria-hidden="true" />
					<span class="font-medium">{time.phrase}</span>
				</span>
				<span class="text-caption text-text-muted tabular-nums">{formatDuration(time)}</span>
				<button
					type="button"
					onclick={() => onremovetime(time.phrase)}
					aria-label={m.editor_time_remove({ phrase: time.phrase })}
					class="{ACTION} hover:text-destructive"
				>
					<X class="size-3.5" aria-hidden="true" />
				</button>
			</li>
		{/each}
		{#each pendingTimes as time (time.phrase)}
			<!-- A proposed time reads in the color of its dotted underline, like a proposed link. -->
			<li class={ROW}>
				<span class="flex min-w-0 items-center gap-1.5 text-time-foreground">
					<Timer class="size-3.5 shrink-0" aria-hidden="true" />
					<span class="font-medium">{time.phrase}</span>
				</span>
				<span class="text-caption text-time-foreground tabular-nums">{formatDuration(time)}</span>
				<span class="flex">
					<button
						type="button"
						onclick={() => onaccepttime(time)}
						aria-label={m.editor_time_accept({ phrase: time.phrase })}
						class="{ACTION} hover:text-primary"
					>
						<Check class="size-3.5" aria-hidden="true" />
					</button>
					<button
						type="button"
						onclick={() => ondismiss(time.phrase)}
						aria-label={m.editor_time_dismiss({ phrase: time.phrase })}
						class="{ACTION} hover:text-destructive"
					>
						<X class="size-3.5" aria-hidden="true" />
					</button>
				</span>
			</li>
		{/each}
	</ul>
{/if}
