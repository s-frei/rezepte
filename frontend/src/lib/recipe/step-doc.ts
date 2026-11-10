import { Extension, type Editor, type Extensions, type JSONContent } from '@tiptap/core';
import Document from '@tiptap/extension-document';
import Paragraph from '@tiptap/extension-paragraph';
import Text from '@tiptap/extension-text';
import { Placeholder, UndoRedo } from '@tiptap/extensions';
import type { Node as ProseMirrorNode } from '@tiptap/pm/model';
import { Plugin, PluginKey } from '@tiptap/pm/state';
import type { StepTime } from '#lib/api/recipes.js';
import type { FormPhoto, FormRef } from './form';
import { firstWordMatch } from './references';

/**
 * The document a step's editor works on: paragraphs of plain text, nothing
 * else.
 *
 * Only these three node extensions are loaded - no starter kit. `steps.text`
 * is a plain string in the database, so bold, italics, headings and lists have
 * nowhere to go: allowing them would let an author format text that is
 * silently thrown away on save. A schema that cannot express them is a
 * stronger guarantee than a toolbar that does not offer them, because it also
 * covers pasting from a word processor and the browser's own Cmd+B.
 */
const STEP_NODES = [
	/**
	 * The document carries what a step's words are, beside its text: the
	 * links, the times and the photo links. Held here rather than only in the form, every
	 * change to them is a transaction like any edit, so the editor's undo
	 * history covers accepting and removing as well as typing, and deleting a word can take its link along in the
	 * same undo step (`dropAbsent`). None of them is rendered: the document's
	 * text is all the editor shows, the decorations draw the rest.
	 */
	Document.extend({
		addAttributes() {
			return {
				references: { default: [], rendered: false },
				times: { default: [], rendered: false },
				photos: { default: [], rendered: false }
			};
		}
	}),
	Paragraph,
	Text
];

/** What a step's words are, as the document carries it. */
export type StepWords = { references: FormRef[]; times: StepTime[]; photos: FormPhoto[] };

export const STEP_WORDS = ['references', 'times', 'photos'] as const;

export function wordsOf(doc: ProseMirrorNode): StepWords {
	const { references, times, photos } = doc.attrs as StepWords;
	return { references, times, photos };
}

/** The document's text, the way `stepText` reads it off an editor. */
function docText(doc: ProseMirrorNode): string {
	return doc.textBetween(0, doc.content.size, '\n');
}

/**
 * Drops whatever names a word the text no longer holds: a link, a time or a
 * photo link. Deleting a linked word ends its link - typing the word
 * again starts from scratch, with at most a proposal - and as an appended
 * transaction it joins the edit's own undo step, so Cmd+Z brings the word and
 * its link back together.
 */
export function dropAbsent(): Plugin {
	return new Plugin({
		key: new PluginKey('stepWordsDropAbsent'),
		appendTransaction(transactions, _before, state) {
			if (!transactions.some((tr) => tr.docChanged)) return null;
			const text = docText(state.doc);
			const has = (word: string) => firstWordMatch(text, word) !== null;
			const words = wordsOf(state.doc);
			const kept: StepWords = {
				references: words.references.filter((ref) => has(ref.word)),
				times: words.times.filter((time) => has(time.phrase)),
				photos: words.photos.filter((photo) => has(photo.phrase))
			};
			let tr = null;
			for (const key of STEP_WORDS) {
				if (kept[key].length !== words[key].length)
					tr = (tr ?? state.tr).setDocAttribute(key, kept[key]);
			}
			return tr;
		}
	});
}

/**
 * Every extension a step's editor loads, built here rather than in the
 * component so the schema the editor really runs on is the one the tests
 * assert against. Asserting on the node list alone would leave the guarantee
 * half-pinned: it is the assembled array that decides what the document can
 * hold.
 *
 * Besides the three nodes there are only two, and neither adds a node or a
 * mark: `UndoRedo` (without it Cmd+Z is dead, which the textarea this replaced
 * never was) and `Placeholder`, which shows the prompt through a decoration
 * rather than by putting text into the document.
 */
export function stepExtensions(placeholder: string, extras: Extensions = []): Extensions {
	return [...STEP_NODES, UndoRedo, Placeholder.configure({ placeholder }), ...extras];
}

/**
 * The per-step extension carrying the decoration plugin and the `@` picker.
 *
 * It is a plain `Extension` holding ProseMirror plugins, and a plugin has no
 * schema of its own - which is precisely why `stepExtensions` can be asserted
 * on: nothing the editor adds on top of the three nodes is able to widen what
 * the document may contain.
 */
export function referenceExtension(plugins: (editor: Editor) => Plugin[]): Extension {
	return Extension.create({
		name: 'ingredientReferences',
		addProseMirrorPlugins() {
			return plugins(this.editor);
		}
	});
}

/**
 * Builds the ProseMirror document for a step's stored text.
 *
 * Deliberately JSON rather than handing the string to Tiptap's `content`
 * option: that option parses its input as HTML, so a step reading
 * `Bei < 100 °C backen` or `Salz & Pfeffer` would come back changed - text the
 * author never touched, rewritten by opening the editor. Every line becomes
 * one paragraph, and `stepText` below joins them back with the same `\n`.
 */
export function textToDoc(text: string, words?: StepWords): JSONContent {
	return {
		type: 'doc',
		...(words ? { attrs: words } : {}),
		content: text
			.split('\n')
			.map((line) =>
				line === ''
					? { type: 'paragraph' }
					: { type: 'paragraph', content: [{ type: 'text', text: line }] }
			)
	};
}

/**
 * The editor's content as the plain text the API stores. `blockSeparator` is
 * `\n`, not Tiptap's default `\n\n`: one paragraph is one line, which is what
 * the textarea this replaced produced and what `textToDoc` reads back.
 */
export function stepText(editor: Editor): string {
	return editor.getText({ blockSeparator: '\n' });
}
