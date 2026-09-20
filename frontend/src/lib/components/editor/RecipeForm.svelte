<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { Pathname, ResolvedPathname } from '$app/types';
	import { toast } from 'svelte-sonner';
	import { ApiError } from '$lib/api/client';
	import type { Image, Recipe, RecipeInput } from '$lib/api/recipes';
	import { session } from '$lib/auth.svelte';
	import ConfirmDialog from '$lib/components/ui/ConfirmDialog.svelte';
	import { m } from '$lib/paraglide/messages';
	import {
		anchorId,
		applyServerErrors,
		cloneForm,
		firstErrorField,
		fromRecipe,
		isDirty,
		toInput,
		validate,
		type FieldErrors
	} from '$lib/recipe/form';
	import { applyDndAriaStrings } from '$lib/recipe/dnd';
	import { shell } from '$lib/shell.svelte';
	import BasicsSection from './BasicsSection.svelte';
	import ImagesSection from './ImagesSection.svelte';
	import IngredientGroupEditor from './IngredientGroupEditor.svelte';
	import SaveBar from './SaveBar.svelte';
	import StepEditor from './StepEditor.svelte';

	let {
		initial,
		heading,
		cancelHref,
		existing,
		save
	}: {
		/** Starting values - `emptyInput()` for a new recipe, the loaded recipe for an edit. */
		initial: RecipeInput;
		heading: string;
		/** Where "Abbrechen" goes. */
		cancelHref: ResolvedPathname;
		/** Set when editing: lets the images section talk to the API for this recipe. */
		existing?: { id: string; images: Image[]; coverImageId: string | null };
		/** Performs the create or update call; `pendingFiles` is non-empty only for a new recipe with queued images. */
		save: (input: RecipeInput, pendingFiles: File[]) => Promise<Recipe>;
	} = $props();

	// `fromRecipe` mints fresh ids, so it runs exactly once: `pristine` is the
	// snapshot the dirty check diffs against, `form` the copy being edited.
	// Seeding the editor from a later `initial` would throw away what the user
	// has typed, so that one-time read is marked with `untrack`.
	const pristine = untrack(() => fromRecipe(initial));
	let form = $state(cloneForm(pristine));
	let errors = $state<FieldErrors>({});
	let saving = $state(false);
	let discardOpen = $state(false);
	// Files the images section has queued for a not-yet-created recipe; the
	// `save` callback uploads them once the recipe exists.
	let pendingFiles = $state<File[]>([]);

	// Not `$state`: they steer a navigation that is already under way, and
	// nothing renders from them.
	let bypassGuard = false;
	let pendingUrl: URL | null = null;

	// svelte-dnd-action announces drags to screen readers in English unless it
	// is handed other strings; this is the only screen that drags anything.
	applyDndAriaStrings();

	const dirty = $derived(isDirty(form, pristine) || pendingFiles.length > 0);

	const sections = [
		{ id: 'editor-section-basics', label: m.editor_section_basics() },
		{ id: 'editor-section-images', label: m.editor_section_images() },
		{ id: 'editor-section-ingredients', label: m.recipe_ingredients() },
		{ id: 'editor-section-steps', label: m.editor_section_steps() }
	];
	let activeSection = $state(sections[0].id);

	const sectionCard = 'scroll-mt-24 rounded-2xl bg-surface p-6 md:p-7';
	const sectionTitle = 'mb-4 font-display text-heading font-medium';

	function scrollToSection(id: string) {
		activeSection = id;
		document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}

	/**
	 * Browsers remember a scrollable element's scroll offset across reloads
	 * (independent of SvelteKit's own scroll restoration), so a chip row
	 * scrolled while testing stays scrolled after a refresh even though the
	 * first chip is active again. Scroll the actually-active one into view
	 * once on mount instead of trusting the restored position - the same
	 * problem `ImageGallery`'s `startAtCover` solves for its swipe strip.
	 */
	function scrollActiveChipIntoView(node: HTMLElement) {
		node.querySelector('[aria-current]')?.scrollIntoView({ inline: 'nearest', block: 'nearest' });
	}

	/** Keeps the section nav in step with what the user has scrolled to. */
	function trackSections(node: HTMLElement) {
		const observer = new IntersectionObserver(
			(entries) => {
				const visible = entries
					.filter((entry) => entry.isIntersecting)
					.sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
				if (visible.length > 0) {
					activeSection = visible[0].target.id;
				}
			},
			// Only the band just under the top bar counts as "current", so the
			// section the user is reading wins over the ones below it.
			{ rootMargin: '-80px 0px -60% 0px' }
		);
		for (const section of node.querySelectorAll('section[id]')) {
			observer.observe(section);
		}
		return () => observer.disconnect();
	}

	async function revealFirstError() {
		const field = firstErrorField(errors);
		if (field === null) {
			return;
		}
		await tick();
		const target = document.getElementById(anchorId(field));
		if (!target) {
			return;
		}
		target.scrollIntoView({ behavior: 'smooth', block: 'center' });
		if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement) {
			target.focus({ preventScroll: true });
		}
	}

	async function handleSave() {
		if (saving) {
			return;
		}
		errors = validate(form);
		if (Object.keys(errors).length > 0) {
			toast.error(m.editor_validation_hint());
			await revealFirstError();
			return;
		}

		saving = true;
		try {
			const recipe = await save(toInput(form), pendingFiles);
			bypassGuard = true;
			toast.success(m.editor_saved());
			await goto(resolve('/recipes/[slug]', { slug: recipe.slug }));
		} catch (error) {
			if (error instanceof ApiError && error.status === 422) {
				errors = applyServerErrors(error.errors);
				if (Object.keys(errors).length > 0) {
					await revealFirstError();
				} else {
					toast.error(error.detail ?? m.editor_save_error());
				}
			} else if (!(error instanceof ApiError && error.status === 401)) {
				// A 401 already sends the browser to the login page (see
				// `$lib/api/client`), so a toast would only flash on the way out.
				toast.error(m.editor_save_error());
			}
		} finally {
			saving = false;
		}
	}

	function handleCancel() {
		void goto(cancelHref);
	}

	function discardChanges() {
		bypassGuard = true;
		const target = pendingUrl;
		pendingUrl = null;
		// `target` is the URL SvelteKit itself was navigating to, so it is
		// already an internal path - the cast only says so to `resolve`, the
		// same way the login page treats its `next` param.
		const href = target
			? resolve(`${target.pathname}${target.search}${target.hash}` as Pathname)
			: cancelHref;
		void goto(href);
	}

	beforeNavigate((navigation) => {
		// A 401 anywhere in the app clears `session.user` and then sends the
		// browser to the login page (see `$lib/api/client`). Holding that
		// navigation back with the discard dialog would strand the user on a
		// form that cannot be saved any more, so the guard steps aside.
		if (session.user === null) {
			return;
		}
		if (!dirty || bypassGuard) {
			return;
		}
		// Leaving the app entirely (reload, closing the tab, an external link)
		// is the `beforeunload` handler's job below - a dialog we render would
		// never be seen.
		if (navigation.type === 'leave') {
			return;
		}
		navigation.cancel();
		pendingUrl = navigation.to?.url ?? null;
		discardOpen = true;
	});

	$effect(() => {
		if (!dirty) {
			return;
		}
		const warn = (event: BeforeUnloadEvent) => event.preventDefault();
		window.addEventListener('beforeunload', warn);
		return () => window.removeEventListener('beforeunload', warn);
	});

	// The editor owns the whole screen, so the top bar's "Importieren" /
	// "+ Neues Rezept" buttons step aside for the breadcrumb (an empty
	// snippet, since `shell.actions` being set is what replaces them).
	$effect(() => {
		shell.breadcrumb = heading;
		shell.actions = noActions;
		return () => {
			shell.breadcrumb = undefined;
			shell.actions = undefined;
		};
	});
</script>

{#snippet noActions()}{/snippet}

<!-- Mobile leaves room for the fixed save bar above the headline. -->
<div class="pt-20 md:pt-10 md:pb-0">
	<h1 class="font-display text-display-sm font-medium md:hidden">{heading}</h1>

	<!-- Mobile: the section list is a horizontally scrollable chip row.
	     The mask fades the right edge so a hidden scrollbar doesn't leave
	     the row's scrollability undiscoverable. -->
	<div
		{@attach scrollActiveChipIntoView}
		class="mt-4 -mr-5 flex [scrollbar-width:none] gap-1.5 overflow-x-auto [mask-image:linear-gradient(to_right,black_calc(100%-32px),transparent)] pr-5 pb-1 md:hidden"
	>
		{#each sections as section (section.id)}
			<button
				type="button"
				onclick={() => scrollToSection(section.id)}
				aria-current={activeSection === section.id ? 'true' : undefined}
				class="flex h-[34px] shrink-0 items-center justify-center rounded-pill px-3.5 text-caption font-semibold transition {activeSection ===
				section.id
					? 'bg-inverse text-inverse-foreground'
					: 'border border-border bg-surface text-text-muted'}"
			>
				{section.label}
			</button>
		{/each}
	</div>

	<div class="mt-5 md:grid md:grid-cols-[200px_minmax(0,720px)] md:items-start md:gap-14">
		<!--
			The rail is one sticky block: the section entries and, under them,
			the save actions. Keeping the actions here rather than over the
			form is what stops them covering anything - see `SaveBar`. The
			wrapper carries no `hidden`, because `SaveBar`'s phone branch is
			`fixed` and renders from inside it; the nav hides itself instead.
		-->
		<div class="md:sticky md:top-6 md:flex md:flex-col md:gap-5">
			<nav
				aria-label={m.editor_sections_label()}
				class="hidden flex-col gap-1.5 text-body-sm font-medium text-text-muted md:flex"
			>
				{#each sections as section (section.id)}
					<button
						type="button"
						onclick={() => scrollToSection(section.id)}
						aria-current={activeSection === section.id ? 'true' : undefined}
						class="rounded-pill px-3.5 py-2 text-left transition {activeSection === section.id
							? 'bg-surface font-semibold text-text'
							: 'hover:text-text'}"
					>
						{section.label}
					</button>
				{/each}
			</nav>

			<SaveBar {dirty} {saving} oncancel={handleCancel} onsave={handleSave} />
		</div>

		<form {@attach trackSections} onsubmit={(event) => event.preventDefault()} class="space-y-5">
			<section id="editor-section-basics" class={sectionCard}>
				<h2 class={sectionTitle}>{m.editor_section_basics()}</h2>
				<BasicsSection bind:form {errors} />
			</section>

			<section id="editor-section-images" class={sectionCard}>
				<h2 class={sectionTitle}>{m.editor_section_images()}</h2>
				<ImagesSection
					recipeId={existing?.id}
					initialImages={existing?.images ?? []}
					initialCoverId={existing?.coverImageId ?? null}
					bind:pending={pendingFiles}
				/>
			</section>

			<section id="editor-section-ingredients" class={sectionCard}>
				<h2 class={sectionTitle}>{m.recipe_ingredients()}</h2>
				{#if errors.ingredientGroups}
					<p class="mb-3 text-micro font-medium text-destructive">{errors.ingredientGroups}</p>
				{/if}
				<IngredientGroupEditor bind:groups={form.ingredientGroups} {errors} />
			</section>

			<section id="editor-section-steps" class={sectionCard}>
				<h2 class={sectionTitle}>{m.editor_section_steps()}</h2>
				{#if errors.steps}
					<p class="mb-3 text-micro font-medium text-destructive">{errors.steps}</p>
				{/if}
				<StepEditor bind:steps={form.steps} />
			</section>
		</form>
	</div>
</div>

<ConfirmDialog
	bind:open={discardOpen}
	title={m.editor_discard_confirm_title()}
	text={m.editor_discard_confirm_text()}
	confirmLabel={m.editor_discard_confirm()}
	destructive
	onconfirm={discardChanges}
/>
