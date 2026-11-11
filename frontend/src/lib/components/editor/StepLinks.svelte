<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import ImageIcon from '@lucide/svelte/icons/image';
	import ImageOff from '@lucide/svelte/icons/image-off';
	import Link2 from '@lucide/svelte/icons/link-2';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Timer from '@lucide/svelte/icons/timer';
	import TimerOff from '@lucide/svelte/icons/timer-off';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	import Unlink from '@lucide/svelte/icons/unlink';
	import { tick } from 'svelte';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';
	import { slide } from 'svelte/transition';
	import type { StepTime } from '#lib/api/recipes.js';
	import { Announcement } from '#lib/announcement.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { FormRef } from '#lib/recipe/form.js';
	import type { PhotoTile } from '#lib/recipe/step-photos.js';
	import {
		guessFirst,
		refLabel,
		refView,
		type Choice,
		type PickerEntry
	} from '#lib/recipe/step-references.js';
	import { ACCEPT, ACTION, ACTIONS, CELL, REMOVE, LIST, ROW } from './link-list';
	import { formatDuration } from '#lib/recipe/times.js';

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
		choices,
		times,
		pendingTimes,
		entries,
		editing,
		onlinkword,
		onmarktime,
		onunlink,
		onaccept,
		onaccepttime,
		onremovetime,
		photos,
		canLinkPhoto,
		onlinkphoto,
		onchangephoto,
		onremovephoto
	}: {
		stepId: string;
		/** The step's links whose word is in the text, in the order they appear there. */
		links: FormRef[];
		/** The matcher's open proposals for the step, in the same order. */
		pending: FormRef[];
		/** Words naming an ingredient in several groups, each listed with every option, in text order. */
		choices: Choice[];
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
		onaccepttime: (time: StepTime) => void;
		onremovetime: (phrase: string) => void;
		/** The step's photo links with the photo each shows, in text order. */
		photos: { phrase: string; tile: PhotoTile }[];
		/** Whether the recipe has a photo to link; "Link photo" is offered only then. */
		canLinkPhoto: boolean;
		/** The buttons open the picker below themselves, so they hand themselves over. */
		onlinkphoto: (trigger: HTMLElement) => void;
		onchangephoto: (phrase: string, trigger: HTMLElement) => void;
		onremovephoto: (phrase: string) => void;
	} = $props();

	let open = $state(false);
	let toggle = $state<HTMLButtonElement | null>(null);
	let list = $state<HTMLElement | null>(null);
	/** What the last button in the list did, for a screen reader. */
	const said = new Announcement();

	/**
	 * Runs a row's button and says what it did. The row it was in goes - an
	 * accepted one moves up among the links, a removed one turns back into a
	 * proposal - so focus would fall to the page. It goes to the button now in
	 * the same place, so a run of accepts needs no walk back down the list;
	 * past the list's end to its last button, and with the list empty to the
	 * summary line.
	 */
	function act(run: () => void, message: string) {
		const buttons = () => [...(list?.querySelectorAll<HTMLElement>('button') ?? [])];
		const at = buttons().indexOf(document.activeElement as HTMLElement);
		run();
		said.say(message);
		void tick().then(() => {
			const now = buttons();
			(now[Math.min(Math.max(at, 0), now.length - 1)] ?? toggle)?.focus();
		});
	}
	const SLIDE_DURATION = $derived(prefersReducedMotion.current ? 0 : 180);

	/** One of a row's buttons: it runs through `act`, which says `said`. */
	type Action = {
		icon: typeof Check;
		label: string;
		accept?: boolean;
		run: () => void;
		said: string;
	};
	/** A link or time row: its name, a note beside it, the amount at the end, one button. */
	type Row = Action & {
		name: string;
		/** The dotted line a proposal carries in the sentence. */
		mark?: 'ref-suggestion' | 'time-proposal';
		note?: string | null;
		amount: string;
		timer?: boolean;
		/** Hung from a choice's word on its guide line rather than a row of the list. */
		nested?: boolean;
	};

	const broken = $derived(links.filter((ref) => !refView(ref, entries).resolved));
	const working = $derived(links.filter((ref) => refView(ref, entries).resolved));
	/** A choice with a guess is a proposal like any other; one without waits for a pick. */
	const waiting = $derived(choices.filter((choice) => choice.guess === null).length);
	const suggested = $derived(pending.length + choices.length - waiting + pendingTimes.length);
	const listed = $derived(working.length + photos.length + times.length + suggested + waiting);

	const summary = $derived(
		[
			working.length + photos.length > 0
				? m.editor_reference_summary_links({ count: working.length + photos.length })
				: '',
			times.length === 1
				? m.editor_time_summary_one()
				: times.length > 1
					? m.editor_time_summary({ count: times.length })
					: '',
			suggested === 1
				? m.editor_reference_summary_suggestions_one()
				: suggested > 1
					? m.editor_reference_summary_suggestions({ count: suggested })
					: '',
			waiting === 1
				? m.editor_reference_summary_waiting_one()
				: waiting > 1
					? m.editor_reference_summary_waiting({ count: waiting })
					: ''
		]
			.filter((part) => part !== '')
			.join(', ')
	);
</script>

<!--
	An action's label, one word below `md:`: the three full labels took two
	lines of a phone. The short word stands within the full label, which stays
	the button's name for a screen reader.
-->
{#snippet actionLabel(full: string, short: string)}
	<span class="max-md:hidden">{full}</span>
	<span class="md:hidden" aria-hidden="true">{short}</span>
	<span class="sr-only md:hidden">{full}</span>
{/snippet}

<!--
    `mousedown` is prevented so the click never takes focus out of the step:
    the selection the button acts on is the one in the editor, and blurring
    the field first would lose sight of it. A picker opens below the button
    that hands itself over.
-->
{#snippet tool(
	Icon: typeof Check,
	full: string,
	short: string,
	run: (trigger: HTMLElement) => void,
	extra: string
)}
	<button
		type="button"
		onmousedown={(event) => event.preventDefault()}
		onclick={(event) => run(event.currentTarget)}
		class="relative flex items-center gap-1 rounded-pill px-1 py-1 text-caption text-text-muted transition after:absolute after:inset-x-0 after:-inset-y-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {extra}"
	>
		<Icon class="size-3.5" aria-hidden="true" />
		{@render actionLabel(full, short)}
	</button>
{/snippet}

{#snippet action(a: Action)}
	<button
		type="button"
		onclick={() => act(a.run, a.said)}
		aria-label={a.label}
		class="{ACTION} {a.accept ? ACCEPT : REMOVE}"
	>
		<a.icon class="size-4" aria-hidden="true" />
	</button>
{/snippet}

<!-- A proposal is marked as in the sentence: its name carries the dotted
     line (`mark`), and nothing else in the row is colored. -->
{#snippet row(r: Row)}
	<li
		class={r.nested
			? 'grid min-h-11 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 pl-3'
			: ROW}
	>
		<span class={CELL}>
			{#if r.timer}
				<span class="flex min-w-0 items-center gap-1.5">
					<Timer class="size-3.5 shrink-0 text-time-foreground" aria-hidden="true" />
					<span class={[r.mark, 'font-medium']}>{r.name}</span>
				</span>
			{:else}
				<!-- Name and group wrap as whole words, so a large text size breaks
				     between them rather than inside the line. -->
				<span class="flex min-w-0 flex-wrap items-baseline gap-x-1.5">
					<span class={[r.mark, 'font-medium']}>{r.name}</span>
					{#if r.note}<span class="text-caption text-text-muted">{r.note}</span>{/if}
				</span>
			{/if}
			<span class="text-caption whitespace-nowrap text-text-muted tabular-nums">{r.amount}</span>
		</span>
		<span class={ACTIONS}>{@render action(r)}</span>
	</li>
{/snippet}

{#snippet link(ref: FormRef, proposed: boolean)}
	{@const view = refView(ref, entries)}
	{@const label = refLabel(ref, entries)}
	{@render row({
		name: view.name,
		mark: proposed ? 'ref-suggestion' : undefined,
		note: view.group,
		amount: view.amount,
		...(proposed
			? {
					icon: Check,
					label: m.editor_reference_accept({ label }),
					accept: true,
					run: () => onaccept(ref),
					said: m.editor_announce_accepted({ label })
				}
			: {
					icon: Unlink,
					label: m.editor_reference_remove({ label }),
					run: () => onunlink(ref.word),
					said: m.editor_announce_removed({ label })
				})
	})}
{/snippet}

{#snippet time(time: StepTime, proposed: boolean)}
	{@render row({
		name: time.phrase,
		mark: proposed ? 'time-proposal' : undefined,
		amount: formatDuration(time),
		timer: true,
		...(proposed
			? {
					icon: Check,
					label: m.editor_time_accept({ phrase: time.phrase }),
					accept: true,
					run: () => onaccepttime(time),
					said: m.editor_announce_accepted({ label: time.phrase })
				}
			: {
					icon: TimerOff,
					label: m.editor_time_remove({ phrase: time.phrase }),
					run: () => onremovetime(time.phrase),
					said: m.editor_announce_removed({ label: time.phrase })
				})
	})}
{/snippet}

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
				<span class="{ACTIONS} ml-auto">
					{@render action({
						icon: Unlink,
						label: m.editor_reference_remove({ label: refLabel(ref, entries) }),
						run: () => onunlink(ref.word),
						said: m.editor_announce_removed({ label: refLabel(ref, entries) })
					})}
				</span>
			</li>
		{/each}
	</ul>
{/if}

{#if editing || listed > 0}
	<!-- `data-links-line`: StepEditor hands back the space it reserves for this line. -->
	<div data-links-line class="mt-1 flex min-h-8 flex-wrap items-center gap-x-2">
		{#if editing}
			{@render tool(
				Link2,
				m.editor_reference_link_word(),
				m.editor_reference_link_word_short(),
				onlinkword,
				'-ml-1 hover:text-primary'
			)}
			{@render tool(
				Timer,
				m.editor_time_mark(),
				m.editor_time_mark_short(),
				onmarktime,
				'hover:text-time-foreground'
			)}
			{#if canLinkPhoto}
				{@render tool(
					ImageIcon,
					m.editor_photo_link(),
					m.editor_photo_link_short(),
					onlinkphoto,
					'hover:text-primary'
				)}
			{/if}
		{/if}
		{#if listed > 0}
			<!--
				`mousedown` is prevented here too: a tap that focused the toggle
				would start editing, the buttons above would push it onto the
				next line of a phone, and the click would land beside it.
			-->
			<button
				bind:this={toggle}
				id="step-links-toggle-{stepId}"
				type="button"
				aria-expanded={open}
				aria-controls="step-links-{stepId}"
				onmousedown={(event) => event.preventDefault()}
				onclick={() => (open = !open)}
				class="relative -mr-1 ml-auto flex items-center gap-1 rounded-pill px-1 py-1 text-caption transition after:absolute after:inset-x-0 after:-inset-y-2 hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary {suggested +
					waiting >
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
		bind:this={list}
		transition:slide={{ duration: SLIDE_DURATION, easing: cubicOut }}
		id="step-links-{stepId}"
		aria-label={m.editor_reference_links()}
		class="{LIST} mt-1 mb-2"
	>
		{#each working as ref (ref.word)}
			{@render link(ref, false)}
		{/each}
		{#each photos as { phrase, tile }, at (phrase)}
			<li class={ROW}>
				<span class="flex min-w-0 items-center gap-2.5">
					<img src={tile.thumb} alt="" class="size-8 shrink-0 rounded-word object-cover" />
					<span id="step-photo-{stepId}-{at}" class="min-w-0 font-medium">{phrase}</span>
				</span>
				<span class={ACTIONS}>
					<button
						type="button"
						onclick={(event) => onchangephoto(phrase, event.currentTarget)}
						aria-label={m.editor_reference_change()}
						aria-describedby="step-photo-{stepId}-{at}"
						class="{ACTION} {ACCEPT}"
					>
						<Pencil class="size-4" aria-hidden="true" />
					</button>
					{@render action({
						icon: ImageOff,
						label: m.editor_photo_remove({ phrase }),
						run: () => onremovephoto(phrase),
						said: m.editor_announce_removed({ label: phrase })
					})}
				</span>
			</li>
		{/each}
		{#each pending as ref (ref.word)}
			{@render link(ref, true)}
		{/each}
		{#each choices as choice (choice.word)}
			<!--
				The word once, on a line of its own, then every group it could mean
				hung from it on a guide line, each with its own "Accept", the guess
				first: one row, so the hairlines frame the whole choice.
			-->
			<li class="px-3.5 py-2 text-body-sm">
				<p class="flex flex-wrap items-baseline gap-x-1.5">
					<span class="ref-suggestion font-medium">{choice.word}</span>
					<span class="text-caption text-text-muted">
						{choice.guess === null
							? m.editor_reference_choice_waiting()
							: m.editor_reference_choice_options({ count: choice.options.length })}
					</span>
				</p>
				<ul class="mt-1 ml-1 list-none border-l-2 border-border">
					{#each guessFirst(choice) as ref (ref.ingredientId)}
						{@const view = refView(ref, entries)}
						{@const label = refLabel(ref, entries)}
						{@render row({
							name: ref.groupName ?? m.editor_reference_choice_no_group(),
							note: view.name !== choice.word ? view.name : null,
							amount: view.amount,
							nested: true,
							icon: Check,
							label: m.editor_reference_accept({ label }),
							accept: true,
							run: () => onaccept(ref),
							said: m.editor_announce_accepted({ label })
						})}
					{/each}
				</ul>
			</li>
		{/each}
		{#each times as entry (entry.phrase)}
			{@render time(entry, false)}
		{/each}
		{#each pendingTimes as entry (entry.phrase)}
			{@render time(entry, true)}
		{/each}
	</ul>
{/if}

<!-- What the list's last button did; the rows themselves go when they are acted on. -->
<p role="status" class="sr-only">{said.text}</p>
