<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { Pathname, ResolvedPathname } from '$app/types';
	import { toast } from 'svelte-sonner';
	import { ApiError } from '$lib/api/client';
	import type { EditPolicy, Image, Recipe, RecipeInput } from '$lib/api/recipes';
	import { session } from '$lib/auth.svelte';
	import ContentsSheet from '$lib/components/nav/ContentsSheet.svelte';
	import RunningHead from '$lib/components/nav/RunningHead.svelte';
	import type { ContentsEntry } from '$lib/components/nav/contents';
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
	import { refusalOr } from '$lib/recipe/access';
	import { applyDndAriaStrings } from '$lib/recipe/dnd';
	import { currentSection, isAtBottom, nextBand, stillPinned } from '$lib/recipe/section-spy';
	import { acceptAll, type DismissedWords } from '$lib/recipe/step-references';
	import { shell } from '$lib/shell.svelte';
	import BasicsSection from './BasicsSection.svelte';
	import EditingSection from './EditingSection.svelte';
	import ImagesSection from './ImagesSection.svelte';
	import IngredientGroupEditor from './IngredientGroupEditor.svelte';
	import SaveBar from './SaveBar.svelte';
	import StepEditor from './StepEditor.svelte';

	let {
		initial,
		heading,
		cancelHref,
		existing,
		access,
		save
	}: {
		/** Starting values - `emptyInput()` for a new recipe, the loaded recipe for an edit. */
		initial: RecipeInput;
		heading: string;
		/** Where "Abbrechen" goes. */
		cancelHref: ResolvedPathname;
		/** Set when editing: lets the images section talk to the API for this recipe. */
		existing?: { id: string; images: Image[]; coverImageId: string | null };
		/** Set when editing an existing recipe; absent means the caller is creating it and is its author. */
		access?: { canChangePolicy: boolean; isAuthor: boolean; authorName: string };
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
	// Proposals the author turned down, keyed by step id. Deliberately not part
	// of `form`: turning one down changes nothing that is saved, so it must not
	// make the form dirty, and it has no business surviving the page.
	let dismissed = $state<DismissedWords>({});

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
		{ id: 'editor-section-steps', label: m.editor_section_steps() },
		{ id: 'editor-section-editing', label: m.editor_section_editing() }
	];
	let activeSection = $state(sections[0].id);

	// The phone's navigation: a running head naming `activeSection`, and the
	// contents sheet it opens. The sheet's summaries read the live form, so
	// it says what the recipe holds right now, not what it held on load.
	let contentsOpen = $state(false);
	// How far the reader is through `activeSection`, 0 to 1 - the running
	// head's hairline. See `measureProgress`.
	let sectionProgress = $state(0);
	// Out of the images section, which owns the photos of a stored recipe.
	let photoCount = $state(0);

	const policyLabels: Record<EditPolicy, string> = {
		default: m.editor_policy_default(),
		open: m.editor_policy_open(),
		locked: m.editor_policy_locked()
	};

	const contents = $derived.by((): ContentsEntry[] => {
		const title = form.title.trim();
		const ingredients = form.ingredientGroups.reduce(
			(n, group) => n + group.ingredients.filter((row) => row.name.trim() !== '').length,
			0
		);
		const steps = form.steps.filter((step) => step.text.trim() !== '').length;
		const summaries = [
			title || m.editor_contents_untitled(),
			String(photoCount),
			String(ingredients),
			String(steps),
			policyLabels[form.editPolicy]
		];
		return sections.map((section, i) => ({
			...section,
			summary: summaries[i],
			placeholder: i === 0 && title === ''
		}));
	});

	// Room for what is pinned over a section once it has been jumped to: the
	// 64px save bar, and on a phone the 50px running head under it.
	const sectionCard = 'scroll-mt-32 rounded-2xl bg-surface p-6 md:scroll-mt-24 md:p-7';
	const sectionTitle = 'mb-4 font-display text-heading font-medium';

	// True while a smooth scroll started by a nav click is running. The
	// sections it passes on the way would otherwise take the highlight away
	// from the one that was clicked. Not `$state`: nothing renders from it.
	let steering = false;
	let steeringTimer: ReturnType<typeof setTimeout> | undefined;
	// Where the window came to rest after that scroll: the clicked section
	// keeps the highlight until the reader scrolls away from here (see
	// `stillPinned`). Not `$state` either.
	let pinnedAt: number | null = null;

	/**
	 * Hands the nav back to the scroll position once the page has been quiet
	 * for `ms`. `scrollend` normally does that first; this covers a click on
	 * the section already in place (no scroll, so no `scrollend`) and
	 * browsers without the event.
	 */
	function releaseSteeringAfter(ms: number) {
		clearTimeout(steeringTimer);
		steeringTimer = setTimeout(endSteering, ms);
	}

	function endSteering() {
		clearTimeout(steeringTimer);
		if (steering) {
			steering = false;
			pinnedAt = window.scrollY;
		}
	}

	function scrollToSection(id: string) {
		activeSection = id;
		steering = true;
		pinnedAt = null;
		releaseSteeringAfter(300);
		document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
		// Also for a pick with nothing to scroll, which fires no scroll events.
		measureProgress();
	}

	/**
	 * Sets `sectionProgress`: how much of the current section has passed
	 * under the running head's bottom edge (128px down, see `sectionCard`).
	 * At the foot of the page every section is as read as it will get.
	 */
	function measureProgress() {
		const section = document.getElementById(activeSection);
		if (!section) {
			return;
		}
		const atBottom = isAtBottom({
			innerHeight: window.innerHeight,
			scrollY: window.scrollY,
			scrollHeight: document.documentElement.scrollHeight
		});
		const passed = 128 - section.getBoundingClientRect().top;
		sectionProgress = atBottom ? 1 : Math.min(1, Math.max(0, passed / section.offsetHeight));
	}

	/** Keeps the section nav in step with what the user has scrolled to. */
	function trackSections(node: HTMLElement) {
		const order = sections.map((section) => section.id);
		// Every section inside the band right now, carried across callbacks
		// (see `nextBand`).
		let inBand: ReadonlySet<string> = new Set();
		// Read once: crossing the breakpoint mid-edit is rare enough not to
		// rebuild the observer for.
		const phone = !window.matchMedia('(min-width: 768px)').matches;

		const update = () => {
			if (steering) {
				return;
			}
			if (stillPinned(pinnedAt, window.scrollY)) {
				return;
			}
			pinnedAt = null;
			activeSection = currentSection(
				order,
				inBand,
				isAtBottom({
					innerHeight: window.innerHeight,
					scrollY: window.scrollY,
					scrollHeight: document.documentElement.scrollHeight
				}),
				activeSection
			);
		};

		const observer = new IntersectionObserver(
			(entries) => {
				inBand = nextBand(
					inBand,
					entries.map((entry) => ({ id: entry.target.id, isIntersecting: entry.isIntersecting }))
				);
				update();
			},
			// Only the band just under the top bar counts as "current", so the
			// section the user is reading wins over the ones below it. On a
			// phone that band starts under the running head too, or the last
			// strip of a card hidden behind it would still name the head.
			{ rootMargin: `-${phone ? 128 : 80}px 0px -60% 0px` }
		);
		for (const section of node.querySelectorAll('section[id]')) {
			observer.observe(section);
		}

		let frame = 0;
		// The observer alone never sees the page reach its end, which is the
		// only way the short last section can become current.
		const onScroll = () => {
			cancelAnimationFrame(frame);
			frame = requestAnimationFrame(measureProgress);
			if (steering) {
				releaseSteeringAfter(150);
				return;
			}
			update();
		};
		const onScrollEnd = () => endSteering();
		window.addEventListener('scroll', onScroll, { passive: true });
		window.addEventListener('scrollend', onScrollEnd);

		return () => {
			observer.disconnect();
			window.removeEventListener('scroll', onScroll);
			window.removeEventListener('scrollend', onScrollEnd);
			clearTimeout(steeringTimer);
			cancelAnimationFrame(frame);
		};
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
		// A step is a ProseMirror surface rather than a textarea, so the test
		// asks what the element can do, not which class it is.
		if (
			target instanceof HTMLInputElement ||
			target instanceof HTMLTextAreaElement ||
			target.isContentEditable
		) {
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

		// What the button's label promises: every proposal still on screen
		// becomes a real reference, so the payload carries what the author saw.
		acceptAll(form.steps, form.ingredientGroups, dismissed);

		saving = true;
		try {
			const recipe = await save(toInput(form), pendingFiles);
			bypassGuard = true;
			toast.success(m.editor_saved());
			await goto(resolve('/recipes/[slug]', { slug: recipe.slug }));
		} catch (error) {
			if (error instanceof ApiError && error.status === 422) {
				errors = applyServerErrors(error.errors, form.steps);
				if (Object.keys(errors).length > 0) {
					await revealFirstError();
				} else {
					toast.error(error.detail ?? m.editor_save_error());
				}
			} else if (!(error instanceof ApiError && error.status === 401)) {
				// A 401 already sends the browser to the login page (see
				// `$lib/api/client`), so a toast would only flash on the way out.
				toast.error(refusalOr(error, m.editor_save_error()));
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

	<!-- Mobile: the running head sticks under the save bar and names the
	     section being read; the contents sheet it opens reaches the others. -->
	<RunningHead
		entries={contents}
		current={activeSection}
		progress={sectionProgress}
		expanded={contentsOpen}
		onopen={() => (contentsOpen = true)}
		class="top-16 mt-2"
	/>
	<ContentsSheet
		bind:open={contentsOpen}
		entries={contents}
		current={activeSection}
		onselect={scrollToSection}
	/>

	<div class="mt-3 md:mt-5 md:grid md:grid-cols-[200px_minmax(0,720px)] md:items-start md:gap-14">
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
					oncount={(count) => (photoCount = count)}
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
				<StepEditor
					bind:steps={form.steps}
					groups={form.ingredientGroups}
					bind:dismissed
					{errors}
				/>
			</section>

			<section id="editor-section-editing" class={sectionCard}>
				<h2 class={sectionTitle}>{m.editor_section_editing()}</h2>
				<EditingSection
					bind:policy={form.editPolicy}
					canChange={access?.canChangePolicy ?? true}
					isAuthor={access?.isAuthor ?? true}
					authorName={access?.authorName ?? ''}
				/>
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
