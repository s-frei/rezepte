<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import Sparkles from '@lucide/svelte/icons/sparkles';
	import Button from '#lib/components/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { copyText } from '#lib/recipe/share.svelte.js';

	// The AI instruction for a recipe or a list, as a card that says what it
	// is: not everyone who imports knows what to do with "an instruction",
	// so a question, one sentence and a real button rather than a link.
	let { kind }: { kind: 'recipe' | 'list' } = $props();

	const text = $derived(kind === 'recipe' ? m.ai_instruction_recipe() : m.ai_instruction_list());
	const title = $derived(
		kind === 'recipe' ? m.ai_instruction_card_title() : m.ai_instruction_card_title_list()
	);
	const explain = $derived(
		kind === 'recipe' ? m.ai_instruction_explain() : m.ai_instruction_explain_list()
	);

	const explainId = $props.id();
	let copied = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	async function copy() {
		if (!(await copyText(text))) return;
		copied = true;
		clearTimeout(timer);
		timer = setTimeout(() => (copied = false), 2000);
	}

	$effect(() => () => clearTimeout(timer));
</script>

<!-- The other of the two warm tones from its ground: `surface` on the
     phone's `background` sheet, `background` in the `surface` dialog. White
     there read as the field above it, not as a card of its own. -->
<div class="rounded-md bg-surface px-3.5 py-3 md:bg-background">
	<p class="flex items-center gap-1.5 text-body-sm font-semibold text-text">
		<Sparkles class="size-4 shrink-0 text-primary" aria-hidden="true" />
		{title}
	</p>
	<p id={explainId} class="mt-1 text-caption text-text-muted">{explain}</p>
	<Button
		variant="secondary"
		size="sm"
		onclick={copy}
		aria-describedby={explainId}
		class="mt-2.5 gap-1.5"
	>
		{#if copied}
			<Check class="size-4 text-success" aria-hidden="true" />
			{m.ai_instruction_copied()}
		{:else}
			<Sparkles class="size-4" aria-hidden="true" />
			{m.ai_instruction_copy()}
		{/if}
	</Button>
</div>
<!-- No toast confirms the copy, so a screen reader hears it here. -->
<span role="status" aria-live="polite" class="sr-only"
	>{copied ? m.ai_instruction_copied() : ''}</span
>
