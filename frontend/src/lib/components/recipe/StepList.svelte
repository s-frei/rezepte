<script lang="ts">
	let { steps }: { steps: string[] } = $props();
</script>

<ol class="flex flex-col gap-[22px]">
	{#each steps as step, index (index)}
		<li class="flex gap-4">
			<span
				aria-hidden="true"
				class="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent font-display text-[16px] font-semibold text-accent-foreground"
			>
				{index + 1}
			</span>
			<p class="pt-1 text-[16px] leading-[1.6] text-text">{step}</p>
		</li>
	{/each}
</ol>
