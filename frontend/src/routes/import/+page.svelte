<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { openImport } from '#lib/import.svelte.js';
	import { sharedInput } from '#lib/recipe/import-input.js';

	// The target of the manifest's share_target and the "Import" shortcut: the
	// overview takes this entry's place, so Back and a reload do not import
	// again, and the dialog opens over it only once that has settled, with the
	// shared link in its field.
	onMount(async () => {
		const shared = sharedInput(page.url.searchParams);
		await goto(resolve('/'), { replace: true });
		openImport(shared);
	});
</script>
