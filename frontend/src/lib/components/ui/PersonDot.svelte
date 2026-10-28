<script lang="ts">
	import { userColorClasses } from '$lib/user/color';

	let {
		color,
		class: className = 'size-3'
	}: {
		/** The person's color; none falls back as `userColorClasses` does. */
		color?: string | null;
		/** Its size, `size-3` unless the place calls for another. */
		class?: string;
	} = $props();
</script>

<!-- A person in their color, beside their name or in place of it. The ring
     keeps a dark color visible on a dark surface, and a light one on a light
     surface, where the dot alone sank into the chip. -->
<span
	aria-hidden="true"
	class="shrink-0 rounded-full ring-[1.5px] ring-text-muted {userColorClasses(color)} {className}"
></span>
