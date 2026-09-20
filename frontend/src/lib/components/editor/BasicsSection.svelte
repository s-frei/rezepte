<script lang="ts">
	import Input from '$lib/components/ui/Input.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import { m } from '$lib/paraglide/messages';
	import type { FieldErrors, RecipeForm } from '$lib/recipe/form';
	import TagInput from './TagInput.svelte';

	let {
		form = $bindable(),
		errors = {}
	}: {
		form: RecipeForm;
		errors?: FieldErrors;
	} = $props();
</script>

<div class="space-y-3.5">
	<Input
		id="editor-title"
		label={m.editor_field_title()}
		bind:value={form.title}
		error={errors.title ?? null}
		required
	/>

	<Textarea
		id="editor-description"
		label={m.editor_field_description()}
		bind:value={form.description}
		error={errors.description ?? null}
		rows={3}
		placeholder={m.editor_description_placeholder()}
	/>

	<!--
		The source gets its own full-width line, under the description and
		above the numbers. A URL is the longest value this form takes, and as
		one of four columns it had about a quarter of the card to show it in -
		too little to tell two recipes from the same site apart. Length is what
		decides the line here, not the field's kind: the three numbers are two
		digits each and share a row happily.
	-->
	<Input
		id="editor-sourceUrl"
		label={m.recipe_source()}
		bind:value={form.sourceUrl}
		error={errors.sourceUrl ?? null}
		type="url"
		inputmode="url"
		placeholder={m.editor_source_placeholder()}
	/>

	<div class="grid gap-3 sm:grid-cols-3">
		<!-- Deliberately `type="text"`: `bind:value` on a number input hands
		     back a number (or null), and every form field here is a string the
		     validation parses itself. `validate` enforces the 1-99 range. -->
		<Input
			id="editor-servings"
			label={m.recipe_servings()}
			bind:value={form.servings}
			error={errors.servings ?? null}
			type="text"
			inputmode="numeric"
		/>
		<Input
			id="editor-prepMinutes"
			label={m.recipe_prep_time()}
			bind:value={form.prepMinutes}
			error={errors.prepMinutes ?? null}
			suffix={m.editor_minutes_suffix()}
			type="text"
			inputmode="numeric"
		/>
		<Input
			id="editor-cookMinutes"
			label={m.recipe_cook_time()}
			bind:value={form.cookMinutes}
			error={errors.cookMinutes ?? null}
			suffix={m.editor_minutes_suffix()}
			type="text"
			inputmode="numeric"
		/>
	</div>

	<TagInput bind:tags={form.tags} error={errors.tags ?? null} />
</div>
