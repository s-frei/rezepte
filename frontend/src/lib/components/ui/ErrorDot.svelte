<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';

	/** The red dot that marks a section holding an error, with its screen reader text. */
	let { class: className = '' }: { class?: string } = $props();
</script>

<span aria-hidden="true" class="size-2 shrink-0 rounded-pill bg-destructive {className}"></span>
<span class="sr-only">{m.editor_section_has_error()}</span>
