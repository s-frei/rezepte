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
	thumb its own width and stop 2px short of the end. The thumb springs a
	hair past its stop and settles, like a toggle that clicks into place.

	The button around the track is the hit area: as tall as a settings row's
	other controls (36px from md, 44px below) and 8px wider on each side,
	given back by negative margins so the track stays where it was drawn:
	sideways all of it, vertically all but a 22px line, so in a top-aligned
	row the track sits on its title's first line instead of 10px below it.
	The focus outline stays on the track.
-->
<BitsSwitch.Root
	{checked}
	onCheckedChange={onchange}
	aria-label={label}
	class="group -mx-2 -my-[11px] inline-flex h-11 shrink-0 items-center px-2 outline-none md:-my-[7px] md:h-9"
>
	<span
		class="inline-flex h-4 w-7 items-center rounded-pill bg-border px-0.5 transition group-focus-visible:outline-2 group-focus-visible:outline-offset-2 group-focus-visible:outline-primary group-data-[state=checked]:bg-primary"
	>
		<BitsSwitch.Thumb
			class="block size-3 rounded-full bg-surface-elevated shadow-card transition-transform duration-200 ease-spring data-[state=checked]:translate-x-3 motion-reduce:transition-none"
		/>
	</span>
</BitsSwitch.Root>
