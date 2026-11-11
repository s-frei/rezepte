<script lang="ts">
	import Scissors from '@lucide/svelte/icons/scissors';
</script>

<!-- The pass-on sheet's cut line above its buttons, scissors on it. -->
<div class="relative border-t-2 border-dashed border-handle" aria-hidden="true">
	<Scissors
		class="absolute -top-2.5 left-5 size-5 -rotate-90 bg-background px-0.5 text-text-muted"
	/>
</div>
