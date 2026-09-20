<script lang="ts">
	import { Switch as BitsSwitch } from 'bits-ui';

	// An on/off setting that takes effect the moment it is flipped - not a
	// choice among several, and not something a form submits later. That is
	// the line between this and `Checkbox`: a checkbox row in this app marks
	// an item in a list (an ingredient, a tag), a switch turns one thing on.
	let {
		checked,
		label,
		onchange
	}: {
		checked: boolean;
		/** Accessible name; the visible text belongs to the caller's own row. */
		label: string;
		onchange: (checked: boolean) => void;
	} = $props();
</script>

<!--
	28x16 track, 12px thumb, 2px of room around it: the numbers come from the
	design system's Switch row. The thumb travels the track's width less its
	own and less that room on both sides, which is the 12px below - written
	out rather than derived, because a `translate-x-full` here would move the
	thumb its own width and stop 2px short of the end.
-->
<BitsSwitch.Root
	{checked}
	onCheckedChange={onchange}
	aria-label={label}
	class="inline-flex h-4 w-7 shrink-0 items-center rounded-pill bg-border px-0.5 transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary data-[state=checked]:bg-primary"
>
	<BitsSwitch.Thumb
		class="block size-3 rounded-full bg-surface-elevated shadow-card transition-transform data-[state=checked]:translate-x-3"
	/>
</BitsSwitch.Root>
