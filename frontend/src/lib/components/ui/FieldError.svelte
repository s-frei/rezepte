<script lang="ts">
	import CircleAlert from '@lucide/svelte/icons/circle-alert';

	/** A field's error line: alert icon and message, as every form shows it. */
	let {
		message,
		id,
		class: className = ''
	}: { message: string; id?: string; class?: string } = $props();
</script>

<p
	{id}
	class="flex items-start gap-1 text-caption font-medium text-destructive motion-safe:animate-error-in {className}"
>
	<CircleAlert class="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
	{message}
</p>
