<script lang="ts">
	import Mail from '@lucide/svelte/icons/mail';

	let {
		id,
		text,
		address = ''
	}: {
		/** For the submit button's aria-describedby, so the note is announced with it. */
		id?: string;
		/** The message, rendered with `\0` where the address goes, so it can stand out. */
		text: string;
		address?: string;
	} = $props();

	const [before, after] = $derived(text.split('\0'));
</script>

<!-- Says what saving the form will mail, and to whom, above its buttons. -->
<div {id} class="flex gap-2.5 rounded-md bg-accent px-3.5 py-3 text-caption text-accent-foreground">
	<Mail class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
	<p class="min-w-0 wrap-break-word">
		{before}{#if after !== undefined}<strong class="font-semibold">{address}</strong>{after}{/if}
	</p>
</div>
