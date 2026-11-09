<script lang="ts">
	import Plus from '@lucide/svelte/icons/plus';
	import { m } from '#lib/paraglide/messages.js';

	/** The login in add mode, resolved by the page (it carries `next`). */
	let { href }: { href: string } = $props();
</script>

<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
<a
	{href}
	class="group grid min-h-[5.5rem] w-full grid-cols-[auto_minmax(0,1fr)] items-center gap-x-3 rounded-index-card border-2 border-dashed border-border p-3 text-text-muted transition hover:border-primary hover:text-primary focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary min-[360px]:gap-x-4"
>
	<span
		class="flex size-15 items-center justify-center rounded-full bg-surface min-[360px]:size-16"
	>
		<Plus class="size-6 transition group-hover:rotate-90" aria-hidden="true" />
	</span>
	<span class="flex min-w-0 flex-col gap-1">
		<span class="font-display text-body-lg font-medium min-[360px]:text-heading"
			>{m.switch_add()}</span
		>
		<span class="truncate text-caption">{m.switch_add_hint()}</span>
	</span>
</a>
