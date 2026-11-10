<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';
	import Plus from '@lucide/svelte/icons/plus';

	/**
	 * The add buttons under the editor's lists (ingredients, steps, groups):
	 * a tinted field with a filled plus dot, as wide as the list it extends.
	 * The caller sets height and any inset change. `pl-3.75` puts the dot where
	 * a field's value starts (1px line + 1px padding + 13px inset). `deep` is
	 * one tint step darker, for the button that adds a whole group.
	 */
	let {
		deep = false,
		class: className = '',
		children,
		...rest
	}: HTMLButtonAttributes & { deep?: boolean; children: Snippet } = $props();
</script>

<button
	type="button"
	class="flex w-full items-center gap-2 rounded-md pr-3 pl-3.75 text-input font-semibold text-accent-foreground transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-45 motion-reduce:transition-none {deep
		? 'bg-accent/80 hover:bg-accent active:bg-accent'
		: 'bg-accent/55 hover:bg-accent/80 active:bg-accent/80'} {className}"
	{...rest}
>
	<span
		class="flex size-6 shrink-0 items-center justify-center rounded-pill bg-primary text-primary-foreground"
	>
		<Plus class="size-3.5" strokeWidth={3} aria-hidden="true" />
	</span>
	{@render children()}
</button>
