<script lang="ts">
	import { Dialog } from 'bits-ui';
	import BottomSheet from '$lib/components/ui/BottomSheet.svelte';
	import { install } from '$lib/install.svelte';
	import { m } from '$lib/paraglide/messages';
	import IosInstallSteps from './IosInstallSteps.svelte';
</script>

<!-- What tapping an install hint opens on an iPhone, which has no install
     dialog of its own: the Safari steps, as the settings card shows them. -->
<BottomSheet bind:open={install.iosSheet} closeLabel={m.common_close()}>
	<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
		<Dialog.Title class="mb-4 font-display text-heading font-medium">
			{m.install_ios_sheet_title()}
		</Dialog.Title>
		<IosInstallSteps />
	</div>
</BottomSheet>
