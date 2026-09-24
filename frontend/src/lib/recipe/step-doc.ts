import { Extension, type Editor, type Extensions, type JSONContent } from '@tiptap/core';
import Document from '@tiptap/extension-document';
import Paragraph from '@tiptap/extension-paragraph';
import Text from '@tiptap/extension-text';
import { Placeholder, UndoRedo } from '@tiptap/extensions';
import type { Plugin } from '@tiptap/pm/state';

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
const STEP_NODES = [Document, Paragraph, Text];

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
export function textToDoc(text: string): JSONContent {
	return {
		type: 'doc',
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
