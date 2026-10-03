<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import X from '@lucide/svelte/icons/x';
	import { m } from '$lib/paraglide/messages';
	import { MAX_TAGS } from '$lib/recipe/form';
	import { acceptSuggestion } from '$lib/recipe/tag-suggestions-import';
	import { ACTION, ROW, TOGGLE } from './link-list';

	/**
	 * An import's keywords that would be new tags, under the tag field the way
	 * a step's link suggestions sit under the step: one quiet line that counts
	 * them and opens into a list, each one taken with ✓ or dropped with ✕.
	 * What is left open is simply not saved, so the collection's tags only
	 * grow when someone agrees to it.
	 */
	let {
		tags = $bindable([]),
		suggestions = $bindable([])
	}: { tags?: string[]; suggestions?: string[] } = $props();

	let open = $state(false);
	const atLimit = $derived(tags.length >= MAX_TAGS);

	function accept(tag: string) {
		({ tags, suggestions } = acceptSuggestion(tags, suggestions, tag, MAX_TAGS));
	}
</script>

{#if suggestions.length > 0}
	<div class="mt-1 flex min-h-8 items-center justify-end">
		<button
			type="button"
			aria-expanded={open}
			aria-controls="tag-suggestions"
			onclick={() => (open = !open)}
			class={TOGGLE}
		>
			{suggestions.length === 1
				? m.editor_reference_summary_suggestions_one()
				: m.editor_reference_summary_suggestions({ count: suggestions.length })}
			<ChevronDown
				class="size-3.5 transition-transform motion-reduce:transition-none {open
					? 'rotate-180'
					: ''}"
				aria-hidden="true"
			/>
		</button>
	</div>
	<!-- Open, the disabled ✓ need their reason beside them, not only in a title. -->
	{#if open && atLimit}
		<p class="mb-1 text-right text-caption text-text-muted">
			{m.editor_tag_limit_reached({ count: MAX_TAGS })}
		</p>
	{/if}
	{#if open}
		<ul
			id="tag-suggestions"
			aria-label={m.editor_tag_suggestions_list()}
			class="list-none divide-y divide-dashed divide-border border-t border-dashed border-border"
		>
			{#each suggestions as tag (tag)}
				<li class={ROW}>
					<span class="min-w-0 break-words">{tag}</span>
					<button
						type="button"
						onclick={() => accept(tag)}
						disabled={atLimit}
						aria-label={m.editor_tag_suggestion_accept({ tag })}
						title={atLimit ? m.editor_tag_limit_reached({ count: MAX_TAGS }) : undefined}
						class="flex size-8 items-center justify-center rounded-pill text-primary transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:opacity-40"
					>
						<Check class="size-4" aria-hidden="true" />
					</button>
					<button
						type="button"
						onclick={() => (suggestions = suggestions.filter((s) => s !== tag))}
						aria-label={m.editor_tag_suggestion_dismiss({ tag })}
						class="{ACTION} hover:text-destructive"
					>
						<X class="size-4" aria-hidden="true" />
					</button>
				</li>
			{/each}
		</ul>
	{/if}
{/if}
