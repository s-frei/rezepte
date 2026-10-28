<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { openImport } from '$lib/import.svelte';
	import { sharedInput } from '$lib/recipe/import-input';

	// The target of the manifest's share_target and the "Import" shortcut: the
	// overview takes this entry's place, so Back and a reload do not import
	// again, and the dialog opens over it only once that has settled - a
	// reading that finished first would navigate to the editor and be undone
	// by the redirect.
	onMount(async () => {
		const shared = sharedInput(page.url.searchParams);
		await goto(resolve('/'), { replaceState: true });
		openImport(shared);
	});
</script>
