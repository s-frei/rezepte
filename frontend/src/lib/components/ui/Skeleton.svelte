<script lang="ts">
	// Base pulse/colour only - size and radius (rounded-*) come from the
	// caller via `class`, since they differ per use (chip, card image, text
	// line, avatar, ...).
	let { class: className = '' }: { class?: string } = $props();
</script>

<div class="animate-pulse bg-skeleton {className}"></div>
