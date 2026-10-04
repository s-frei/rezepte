<script lang="ts">
	import { overLimit } from '$lib/recipe/diary';
	import { m } from '$lib/paraglide/messages';

	let {
		value = $bindable(''),
		element = $bindable(),
		label,
		describedby,
		oncancel,
		plain = false,
		class: extra = ''
	}: {
		value?: string;
		element?: HTMLTextAreaElement;
		/** The accessible name, also shown as the placeholder of an empty field. */
		label: string;
		/** The id of a hint that explains the keys. */
		describedby?: string;
		/** Escape: leave the edit without saving. */
		oncancel?: () => void;
		/** Inside the composer's box, which draws the edge and the focus ring. */
		plain?: boolean;
		/** Placement in the caller's form row. */
		class?: string;
	} = $props();

	const over = $derived(overLimit(value));
	// The limit message also says why sending is off.
	const overId = $props.id();

	function keydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && oncancel) {
			event.preventDefault();
			oncancel();
			return;
		}
		// An IME's Enter only confirms its composition (Safari reports that
		// one as keyCode 229 with isComposing false).
		if (event.key !== 'Enter' || event.isComposing || event.keyCode === 229) return;
		// With a keyboard Enter writes and Shift+Enter breaks the line. A
		// touch keyboard has no Shift+Enter, so there Enter breaks the line
		// and the send button writes. Cmd/Ctrl+Enter writes everywhere.
		const touch = matchMedia('(pointer: coarse)').matches;
		if (event.metaKey || event.ctrlKey || (!event.shiftKey && !touch)) {
			event.preventDefault();
			element?.form?.requestSubmit();
		}
	}
</script>

<!--
	The field of the composer for a new entry and the in-place editor of an
	existing one, 26px a line. The editor is ruled like the bubbles - the rule
	spacing of `diary-paper`, so the two must change together.
-->
<label class="min-w-0 flex-1 {extra}">
	<span class="sr-only">{label}</span>
	<textarea
		bind:this={element}
		bind:value
		onkeydown={keydown}
		rows="1"
		placeholder={label}
		aria-describedby={[describedby, over > 0 && overId].filter(Boolean).join(' ') || undefined}
		aria-invalid={over > 0}
		class="block field-sizing-content max-h-[calc(6*26px+14px)] w-full min-w-0 resize-none font-display text-card-sm leading-[26px] wrap-anywhere text-text italic placeholder:text-text-muted {plain
			? 'bg-transparent py-[3px] outline-none md:py-1'
			: 'min-h-10 rounded-2xl border border-border bg-surface-elevated diary-paper px-4 pt-1.5 pb-2 focus-visible:outline-2 focus-visible:outline-primary'}"
	></textarea>
</label>
{#if over > 0}
	<p id={overId} class="order-last w-full text-caption text-destructive">
		{m.diary_too_long({ count: over })}
	</p>
{/if}
