<script lang="ts">
	import Plus from '@lucide/svelte/icons/plus';
	import { m } from '#lib/paraglide/messages.js';

	let {
		onadd,
		large = false,
		class: className = ''
	}: {
		/** Stores the text; resolves to whether it landed, which clears the field. */
		onadd: (text: string) => Promise<boolean>;
		/** The shopping mode's 56px field and button. */
		large?: boolean;
		class?: string;
	} = $props();

	let text = $state('');
	let busy = $state(false);
	let field: HTMLInputElement | undefined = $state();

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		const trimmed = text.trim();
		if (!trimmed || busy) return;
		busy = true;
		try {
			if (await onadd(trimmed)) text = '';
		} finally {
			busy = false;
			// The + button took the focus; the next item is typed right away.
			field?.focus();
		}
	}
</script>

<form onsubmit={submit} class="flex min-w-0 items-center {large ? 'gap-2.5' : 'gap-2'} {className}">
	<input
		bind:this={field}
		bind:value={text}
		aria-label={m.list_add_placeholder()}
		placeholder={m.list_add_placeholder()}
		autocomplete="off"
		enterkeyhint="done"
		maxlength={200}
		class="min-w-0 flex-1 truncate rounded-pill border border-border bg-surface-elevated {large
			? 'h-14 px-5'
			: 'h-11 px-4.5'} text-body outline-none placeholder:text-text-muted focus:border-primary"
	/>
	<button
		type="submit"
		aria-label={m.list_add()}
		class="inline-flex shrink-0 {large
			? 'size-14'
			: 'size-11'} items-center justify-center rounded-full bg-primary hover-tint text-primary-foreground transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98]"
	>
		<Plus class={large ? 'size-[22px]' : 'size-5'} aria-hidden="true" />
	</button>
</form>
