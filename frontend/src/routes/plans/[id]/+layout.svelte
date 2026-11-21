<script lang="ts">
	import { page } from '$app/state';

	let { children } = $props();
</script>

<!-- Another plan is a fresh page: a stamp leads from plan to plan, and the
     week's folds, dialogs and history must not carry over. -->
{#key page.params.id}
	{@render children()}
{/key}
