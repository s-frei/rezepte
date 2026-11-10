<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { beforeNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { resolvePath } from '#lib/paths.js';
	import type { Path, ResolvedPathname } from '$app/types';
	import FieldError from '#lib/components/ui/FieldError.svelte';
	import ErrorDot from '#lib/components/ui/ErrorDot.svelte';
	import { toast } from 'svelte-sonner';
	import { ApiError } from '#lib/api/client.js';
	import {
		MAX_IMAGES,
		type EditPolicy,
		type Image,
		type Recipe,
		type RecipeInput
	} from '#lib/api/recipes.js';
	import { session } from '#lib/auth.svelte.js';
	import ContentsSheet from '#lib/components/nav/ContentsSheet.svelte';
	import type { ContentsEntry } from '#lib/components/nav/contents.js';
	import ConfirmDialog from '#lib/components/ui/ConfirmDialog.svelte';
	import RequiredMark, { requiredState } from '#lib/components/ui/RequiredMark.svelte';
	import { withoutErrors } from '#lib/form-errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import {
		applyReview,
		applyServerErrors,
		currentErrors,
		errorAnchors,
		fromRecipe,
		hasNamedIngredient,
		isDirty,
		newId,
		photoErrors,
		sectionsWithErrors,
		toInput,
		validate,
		type FieldErrors
	} from '#lib/recipe/form.js';
	import { refusalOr } from '#lib/recipe/access.js';
	import { photoIds, type PhotoTile, type QueuedPhoto } from '#lib/recipe/step-photos.js';
	import { applyDndAriaStrings } from '#lib/recipe/dnd.js';
	import { takeDraft, type ImportedDraft } from '#lib/recipe/draft.js';
	import { currentSection, isAtBottom, nextBand, stillPinned } from '#lib/recipe/section-spy.js';
	import { shell } from '#lib/shell.svelte.js';
	import BasicsSection from './BasicsSection.svelte';
	import EditorBar from './EditorBar.svelte';
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
		save,
		savedHref = (recipe) => resolve('/recipes/[slug]', { slug: recipe.slug }),
		imported
	}: {
		/** Starting values - `emptyInput()` for a new recipe, the loaded recipe for an edit. */
		initial: RecipeInput;
		heading: string;
		/** Where "Abbrechen" goes. */
		cancelHref: ResolvedPathname;
		/** Set when editing: lets the images section talk to the API for this recipe. */
		existing?: { id: string; images: Image[] };
		/** Set when editing an existing recipe; absent means the caller is creating it and is its author. */
		access?: { canChangePolicy: boolean; isAuthor: boolean; authorName: string };
		/**
		 * Performs the create or update call; `pendingFiles` is non-empty only
		 * for a new recipe with queued images. A step's photo link to a queued
		 * one carries that photo's `key` as its `imageId` until it is uploaded.
		 */
		save: (input: RecipeInput, pendingFiles: QueuedPhoto[]) => Promise<Recipe>;
		/** Where a successful save goes; the recipe's page unless the caller says otherwise. */
		savedHref?: (recipe: Recipe) => ResolvedPathname;
		/** Set when the recipe comes from an import: hints, tag suggestions and the queued photo. */
		imported?: ImportedDraft;
	} = $props();

	// `fromRecipe` mints fresh ids, so it runs exactly once: `pristine` is the
	// snapshot the dirty check diffs against, `form` the copy being edited.
	// Seeding the editor from a later `initial` would throw away what the user
	// has typed, so that one-time read is marked with `untrack`.
	const pristine = untrack(() => applyReview(fromRecipe(initial), imported?.draft.review ?? []));
	let form = $state(structuredClone(pristine));
	// What the last save reported, less what has been typed away since.
	let saveErrors = $state<FieldErrors>({});
	// Of those, the ones that still hold: a link error leaves the moment its
	// link, step or row is fixed, which no typing has to announce (see
	// `currentErrors`). Everything on screen reads this one.
	const errors = $derived(currentErrors(saveErrors, form));
	let saving = $state(false);
	// The save went through and the save button shows it (see `SAVED_PAUSE`).
	let saved = $state(false);
	let discardOpen = $state(false);
	// The navigation held back is a Share → Rezepte, so the question names it.
	let discardForShare = $state(false);
	// Files the images section has queued for a not-yet-created recipe; the
	// `save` callback uploads them once the recipe exists.
	let pendingFiles = $state<QueuedPhoto[]>(
		untrack(() => (imported?.photo ? [{ key: newId(), file: imported.photo }] : []))
	);
	// An import's keywords that would be new tags; not part of
	// `form`, since only the accepted ones are saved.
	let suggestedTags = $state<string[]>(untrack(() => imported?.draft.suggestedTags ?? []));

	// Not `$state`: they steer a navigation that is already under way, and
	// nothing renders from them.
	let bypassGuard = false;
	let pendingUrl: URL | null = null;

	// svelte-dnd-action announces drags to screen readers in English unless it
	// is handed other strings; this is the only screen that drags anything.
	applyDndAriaStrings();

	// An unsaved import is worth guarding before any edit.
	const dirty = $derived(
		isDirty(form, pristine) || pendingFiles.length > 0 || imported !== undefined
	);

	const sections = [
		{ id: 'editor-section-basics', label: m.editor_section_basics() },
		{ id: 'editor-section-images', label: m.editor_section_images() },
		{ id: 'editor-section-ingredients', label: m.recipe_ingredients() },
		{ id: 'editor-section-steps', label: m.editor_section_steps() },
		{ id: 'editor-section-editing', label: m.editor_section_editing() }
	];
	let activeSection = $state(sections[0].id);

	// The phone's navigation: `EditorBar` naming `activeSection`, and the
	// contents sheet it opens. The sheet's summaries read the live form, so
	// it says what the recipe holds right now, not what it held on load.
	let contentsOpen = $state(false);
	// How far the reader is through `activeSection`, 0 to 1 - the segments
	// under the name in `EditorBar`. See `measureProgress`.
	let sectionProgress = $state(0);
	// Out of the images section, which owns the photos of a stored recipe.
	let photoCount = $state(0);
	// Its pending deletes are sent before leaving, or the next page still shows them.
	let images = $state<ImagesSection>();
	// The photos a step may link to, as the images section shows them.
	let photoTiles = $state<PhotoTile[]>([]);
	// The counter's words around the count, which is set in bold.
	const photoCounter = m.images_counter({ count: '\u0000', max: MAX_IMAGES }).split('\u0000');

	const policyLabels: Record<EditPolicy, string> = {
		default: m.editor_policy_default(),
		open: m.editor_policy_open(),
		locked: m.editor_policy_locked()
	};

	// Sections holding an error of the last save, by element id: their
	// segment in `EditorBar`, their entry in the contents sheet and their rail
	// button carry a destructive mark until the errors are edited away.
	const errorIds = $derived(
		new Set([...sectionsWithErrors(errors)].map((section) => `editor-section-${section}`))
	);

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
			placeholder: i === 0 && title === '',
			error: errorIds.has(section.id)
		}));
	});

	// Room for what is pinned over a section once it has been jumped to: on a
	// phone the 60px `EditorBar` (its `scroll-padding-top` on the page) plus
	// this breath, `PINNED` together. Tighter cards on a phone, where every
	// pixel of width is a pixel of field.
	const sectionCard = 'scroll-mt-4 rounded-2xl bg-surface p-3 md:scroll-mt-24 md:p-7';
	const sectionTitle = 'mb-3 font-display text-heading font-medium md:mb-4';
	// The bar's 60px and `scroll-mt-4`: where a section counts as read past on a phone.
	const PINNED = 76;

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

	/**
	 * Names section `id` in the nav now and holds it there while a scroll
	 * started by the app carries the page to it, so the sections passed on
	 * the way - or one that only ends up in the spy band - do not take over.
	 */
	function steer(id: string) {
		activeSection = id;
		steering = true;
		pinnedAt = null;
		releaseSteeringAfter(300);
	}

	function scrollToSection(id: string) {
		steer(id);
		document.getElementById(id)?.scrollIntoView({
			behavior: prefersReducedMotion.current ? 'auto' : 'smooth',
			block: 'start'
		});
		// Also for a pick with nothing to scroll, which fires no scroll events.
		measureProgress();
	}

	/**
	 * Sets `sectionProgress`: how much of the current section has passed
	 * under `EditorBar`'s bottom edge (`PINNED`, see `sectionCard`).
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
		const passed = PINNED - section.getBoundingClientRect().top;
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
			// phone that band starts under `EditorBar`, or the last strip of a
			// card hidden behind it would still name the bar.
			{ rootMargin: `-${phone ? PINNED : 80}px 0px -60% 0px` }
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

	// The anchor the last reveal went to, so "Next" in the toast goes on
	// from there. Not `$state`: nothing renders from it.
	let revealed: string | undefined;

	/**
	 * Scrolls the field with error anchor `id` into view and focuses it. The
	 * nav is steered to the section holding it: a centered field can leave
	 * another section in the spy band, and the bar would name that one while
	 * the red segment is this one's. A section-level error's anchor is the
	 * section itself, which goes to the top like a jump from the nav.
	 */
	async function revealAnchor(id: string) {
		revealed = id;
		await tick();
		const target = document.getElementById(id);
		if (!target) {
			return;
		}
		const section = target.closest<HTMLElement>('section[id]');
		if (section) {
			steer(section.id);
		}
		target.scrollIntoView({
			behavior: prefersReducedMotion.current ? 'auto' : 'smooth',
			block: target === section ? 'start' : 'center'
		});
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

	/**
	 * The error after the one revealed last, cycling; the first when that
	 * one has been edited away.
	 */
	function revealNextError() {
		const anchors = errorAnchors(errors);
		if (anchors.length === 0) {
			return;
		}
		const at = revealed === undefined ? -1 : anchors.indexOf(revealed);
		void revealAnchor(anchors[(at + 1) % anchors.length]);
	}

	/**
	 * Says how many fields need something, with a way to the next one, and
	 * reveals the first. The count is taken now; the toast does not follow
	 * edits - the destructive marks in the bar, sheet and rail do.
	 */
	async function reportErrors() {
		const anchors = errorAnchors(errors);
		const count = anchors.length;
		toast.error(count === 1 ? m.editor_errors_count_one() : m.editor_errors_count({ count }), {
			duration: 8000,
			action: {
				label: m.editor_errors_next(),
				onClick: (event) => {
					// Kept open, so "Next" can be pressed again to walk the errors.
					event.preventDefault();
					revealNextError();
				}
			}
		});
		await revealAnchor(anchors[0]);
	}

	// An error answers the last save; once its field is edited it no longer
	// describes what is there, so it goes. Fields name the errors they own
	// with `data-error-key` (space-separated - an ingredient's name also
	// answers the section's "at least one"), which lets this one listener on
	// the form serve every field without each section wiring its own.
	function clearEditedErrors(event: Event) {
		const keys =
			event.target instanceof HTMLElement
				? event.target.closest<HTMLElement>('[data-error-key]')?.dataset.errorKey
				: undefined;
		if (keys) {
			saveErrors = withoutErrors(errors, keys.split(' '));
		}
	}

	// How long the save button stays green before the editor moves on: the
	// ring's 420ms (`--animate-saved-ring`), or under reduced motion, which
	// shows the color with neither ring nor drawing, just long enough to see.
	const SAVED_PAUSE = $derived(prefersReducedMotion.current ? 250 : 420);

	async function handleSave() {
		if (saving || saved) {
			return;
		}
		saveErrors = validate(form);
		if (Object.keys(errors).length > 0) {
			await reportErrors();
			return;
		}

		// Open proposals are not taken: only what the author accepted is
		// linked, so nothing unreviewed reaches a cook. They come back as
		// proposals the next time the recipe is edited.
		saving = true;
		try {
			// Uploads still going up first, so every link names a photo the
			// server holds. A removal waiting for its undo toast stays undoable
			// until the save went through: its link is not sent anyway. A
			// queued photo of a new recipe is named by its key, which `save`
			// resolves once it is uploaded.
			await images?.settle();
			await tick();
			const ids = photoIds(photoTiles, existing === undefined);
			// A linked photo whose upload failed, or one a retry started after
			// the wait, stops the save under its step, so the author can retry
			// or remove it instead of losing the link unseen.
			const unsent = new Set([
				...(images?.failedKeys() ?? []),
				...photoTiles.filter((tile) => !ids.has(tile.key)).map((tile) => tile.key)
			]);
			saveErrors = photoErrors(form, ids, unsent);
			if (Object.keys(errors).length > 0) {
				await reportErrors();
				return;
			}
			const recipe = await save(toInput(form, ids), pendingFiles);
			bypassGuard = true;
			toast.success(m.editor_saved());
			// A removed photo's link was left out of the save; a delete that
			// then fails keeps the photo, and the author learns the link went.
			const kept = (await images?.flush()) ?? [];
			if (form.steps.some((step) => step.photos.some((photo) => kept.includes(photo.imageKey)))) {
				toast.error(m.editor_photo_link_dropped());
			}
			// The page turns inert with it, so nothing is typed or pressed
			// into a recipe that is already saved.
			saving = false;
			saved = true;
			await new Promise((done) => setTimeout(done, SAVED_PAUSE));
			await goto(savedHref(recipe));
		} catch (error) {
			if (error instanceof ApiError && error.status === 422) {
				saveErrors = applyServerErrors(error.errors, form.steps);
				if (Object.keys(errors).length > 0) {
					await reportErrors();
				} else {
					toast.error(error.detail ?? m.editor_save_error());
				}
			} else if (!(error instanceof ApiError && error.status === 401)) {
				// A 401 already sends the browser to the login page (see
				// `#lib/api/client`), so a toast would only flash on the way out.
				toast.error(refusalOr(error, m.editor_save_error()));
			}
		} finally {
			saving = false;
		}
	}

	async function handleCancel() {
		// A dirty form asks first: `discardChanges` flushes on the way out.
		if (!dirty) await flushImages();
		void goto(cancelHref);
	}

	/** Waits for the photos still going up or out, with the buttons busy meanwhile. */
	async function flushImages() {
		saving = true;
		await images?.flush();
		saving = false;
	}

	async function discardChanges() {
		bypassGuard = true;
		const target = pendingUrl;
		pendingUrl = null;
		// `target` is the URL SvelteKit itself was navigating to, so it is
		// already an internal path - the cast only says so to `resolve`, the
		// same way the login page treats its `next` param.
		const href = target
			? resolvePath(`${target.pathname.slice(1)}${target.search}${target.hash}` as Path)
			: cancelHref;
		await flushImages();
		void goto(href);
	}

	beforeNavigate((navigation) => {
		// A 401 anywhere in the app clears `session.user` and then sends the
		// browser to the login page (see `#lib/api/client`). Holding that
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
		discardForShare = navigation.to?.route.id === '/import';
		discardOpen = true;
	});

	// The discard dialog closed without "Discard" (which clears `pendingUrl`
	// first): the navigation is given up, and an import riding on it with it.
	$effect(() => {
		if (discardOpen || pendingUrl === null) {
			return;
		}
		pendingUrl = null;
		takeDraft(); // drops it
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

<!-- A phone's top is `EditorBar`, pinned and 60px high; the page starts under it. -->
<div class="pt-15 md:pt-10 md:pb-0" inert={saved}>
	<!-- No visible headline: on a phone the bar and the first card say where
	     you are, on desktop the top bar's breadcrumb does. It stays on every
	     width for assistive tech, so the page still has a heading. -->
	<h1 class="sr-only">{heading}</h1>

	<EditorBar
		entries={contents}
		current={activeSection}
		progress={sectionProgress}
		dirty={dirty && !saved}
		{saving}
		{saved}
		{errorIds}
		expanded={contentsOpen}
		onopen={() => {
			// A modal sheet under a toast reads as broken, and the sheet marks
			// the sections with errors itself.
			toast.dismiss();
			contentsOpen = true;
		}}
		oncancel={handleCancel}
		onsave={handleSave}
	/>
	<ContentsSheet
		bind:open={contentsOpen}
		entries={contents}
		current={activeSection}
		onselect={scrollToSection}
	/>

	<div class="mt-3 md:mt-5 md:grid md:grid-cols-[200px_minmax(0,720px)] md:items-start md:gap-14">
		<!--
			Desktop: the rail is one sticky block, the section entries and,
			under them, the save actions. Keeping the actions here rather than
			over the form is what stops them covering anything - see `SaveBar`.
		-->
		<div class="hidden md:sticky md:top-6 md:flex md:flex-col md:gap-5">
			<nav
				aria-label={m.editor_sections_label()}
				class="flex flex-col gap-1.5 text-body-sm font-medium text-text-muted"
			>
				{#each sections as section (section.id)}
					<button
						type="button"
						onclick={() => scrollToSection(section.id)}
						aria-current={activeSection === section.id ? 'true' : undefined}
						class="flex items-center gap-2 rounded-pill px-3.5 py-2 text-left transition {activeSection ===
						section.id
							? 'bg-surface font-semibold text-text'
							: 'hover:text-text'}"
					>
						{section.label}
						{#if errorIds.has(section.id)}
							<ErrorDot />
						{/if}
					</button>
				{/each}
			</nav>

			<SaveBar
				dirty={dirty && !saved}
				{saving}
				{saved}
				oncancel={handleCancel}
				onsave={handleSave}
			/>
		</div>

		<form
			{@attach trackSections}
			onsubmit={(event) => event.preventDefault()}
			oninput={clearEditedErrors}
			class="space-y-3 max-md:-mx-2 md:space-y-5"
		>
			<section id="editor-section-basics" class={sectionCard}>
				<!-- The legend sits once, by the first section, where the first
				     star is, and keeps its star when the fields have turned to
				     checks: it explains the star. Hidden from assistive tech,
				     which hears `aria-required` on each field and has no star to
				     decode. -->
				<div class="mb-3 flex items-baseline justify-between gap-3 md:mb-4">
					<h2 class="font-display text-heading font-medium">{m.editor_section_basics()}</h2>
					<p aria-hidden="true" class="text-micro text-text-muted">
						<RequiredMark state="empty" />
						{m.editor_required_legend()}
					</p>
				</div>
				<BasicsSection bind:form {errors} bind:suggestedTags />
			</section>

			<!-- The whole card takes dropped photos: `ImagesSection` sets `data-dropping`. -->
			<section
				id="editor-section-images"
				class="{sectionCard} data-dropping:bg-accent/45 data-dropping:outline-2 data-dropping:-outline-offset-2 data-dropping:outline-primary data-dropping:outline-dashed"
			>
				<h2 class="{sectionTitle} flex items-baseline justify-between gap-3">
					{m.editor_section_images()}
					<span class="font-sans text-caption font-normal text-text-muted tabular-nums">
						{photoCounter[0]}<b class="font-semibold text-text">{photoCount}</b>{photoCounter[1]}
					</span>
				</h2>
				<ImagesSection
					bind:this={images}
					recipeId={existing?.id}
					title={form.title}
					initialImages={existing?.images ?? []}
					bind:pending={pendingFiles}
					bind:visible={photoTiles}
					oncount={(count) => (photoCount = count)}
				/>
			</section>

			<section id="editor-section-ingredients" class={sectionCard}>
				<!-- The section needs one named ingredient, not any row in
				     particular, so the star sits on the heading. No margin
				     stroke: there is no single field it could stand beside. -->
				<h2 class={sectionTitle}>
					{m.recipe_ingredients()}
					<RequiredMark
						state={requiredState(
							hasNamedIngredient(form.ingredientGroups),
							!!errors.ingredientGroups
						)}
					/>
				</h2>
				{#if errors.ingredientGroups}
					<FieldError message={errors.ingredientGroups} class="mb-3" />
				{/if}
				<IngredientGroupEditor bind:groups={form.ingredientGroups} {errors} />
			</section>

			<section id="editor-section-steps" class={sectionCard}>
				<h2 class={sectionTitle}>{m.editor_section_steps()}</h2>
				{#if errors.steps}
					<FieldError message={errors.steps} class="mb-3" />
				{/if}
				<StepEditor
					bind:steps={form.steps}
					groups={form.ingredientGroups}
					tiles={photoTiles}
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
	title={discardForShare ? m.editor_discard_share_title() : m.editor_discard_confirm_title()}
	text={m.editor_discard_confirm_text()}
	confirmLabel={m.editor_discard_confirm()}
	cancelLabel={discardForShare ? m.editor_discard_share_keep() : undefined}
	destructive
	onconfirm={discardChanges}
/>
