<script lang="ts">
	let {
		label,
		first = false,
		class: className = ''
	}: {
		label: string;
		/** The first group sits under the list's own head and needs no air above. */
		first?: boolean;
		/** The inset, matching the rows of the list it heads. */
		class?: string;
	} = $props();
</script>

<!--
	A group's heading in a nav list: a small label and a hairline running on
	to the edge, heading and separator in one. Small, sans and muted, so it
	never reads as one more entry beside the larger rows it orders, and not in
	`primary`, which marks the current entry. More air above than below ties
	it to its own rows, and the 8px under it keeps a current or hovered row's
	pill off the label. Screen readers get the name from the group itself.
-->
<p
	aria-hidden="true"
	class="flex items-center gap-2.5 pb-2 text-micro font-semibold tracking-wide text-text-muted {first
		? 'pt-1'
		: 'pt-5.5'} {className}"
>
	{label}
	<span class="h-px flex-1 bg-border"></span>
</p>
