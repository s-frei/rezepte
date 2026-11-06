<script lang="ts">
	import type { Snippet } from 'svelte';

	let { label, children }: { label: string; children: Snippet } = $props();

	const uid = $props.id();
</script>

<!--
	A labeled run of `SettingsRow`s, headed like the settings nav's groups: a
	small muted label and a hairline running on to the edge. The rows are
	split by the dashed hairline the people list uses, none after the last.
-->
<div role="group" aria-labelledby={uid}>
	<h3 id={uid} class="flex items-center gap-3 text-caption font-semibold text-text-muted">
		{label}
		<span aria-hidden="true" class="h-px flex-1 bg-border"></span>
	</h3>
	<div class="divide-y divide-dashed divide-border">
		{@render children()}
	</div>
</div>
