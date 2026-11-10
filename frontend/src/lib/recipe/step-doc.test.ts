import { describe, expect, it } from 'vitest';
import { generateText, getSchema } from '@tiptap/core';
import { Node as ProseMirrorNode } from '@tiptap/pm/model';
import { EditorState, type Transaction } from '@tiptap/pm/state';
import { dropAbsent, referenceExtension, stepExtensions, textToDoc, wordsOf } from './step-doc';

/**
 * The extensions a step's editor really runs on - the placeholder text and the
 * reference plugins are the only things the component adds, and both are
 * passed here so the schema under test is the schema in the browser.
 */
const EXTENSIONS = stepExtensions('Was ist zu tun?', [referenceExtension(() => [])]);

/**
 * `generateText` builds the document from the schema and reads it back with no
 * browser involved, which is exactly the round trip the editor performs when a
 * recipe is opened and saved again.
 */
function roundTrip(text: string): string {
	return generateText(textToDoc(text), EXTENSIONS, { blockSeparator: '\n' });
}

describe('textToDoc', () => {
	it('gives back the author’s text unchanged', () => {
		for (const text of [
			'Zucker und Mehl verrühren.',
			'Bei < 100 °C backen & ruhen lassen.',
			'Erste Zeile\nZweite Zeile',
			'Absatz\n\nNoch einer',
			'  zwei Leerzeichen vorn und hinten  ',
			'Äpfel schälen – dann vierteln.',
			''
		]) {
			expect(roundTrip(text)).toBe(text);
		}
	});

	it('keeps a blank line as a line rather than dropping it', () => {
		// The one case a naive "filter out empty paragraphs" gets wrong.
		expect(textToDoc('a\n\nb').content).toHaveLength(3);
		expect(roundTrip('a\n\nb')).toBe('a\n\nb');
	});
});

describe('stepExtensions', () => {
	it('cannot express any formatting', () => {
		// The guarantee behind `steps.text` being a plain string: there is no
		// mark to carry bold or italics and no node but a paragraph, so nothing
		// a paste or a browser shortcut produces can survive into the document
		// and then be silently dropped on save. Asserted on the assembled array
		// rather than on the three nodes alone, so adding a starter kit - or any
		// one mark - to what the editor loads breaks this test.
		const schema = getSchema(EXTENSIONS);
		expect(Object.keys(schema.marks)).toEqual([]);
		expect(Object.keys(schema.nodes).sort()).toEqual(['doc', 'paragraph', 'text']);
	});
});

describe('dropAbsent', () => {
	const schema = getSchema(EXTENSIONS);
	const words = {
		references: [{ word: 'Teig', ingredientId: 'r1', groupName: null, ingredientName: 'Teig' }],
		times: [],
		photos: [{ phrase: 'Salzwasser schaben', imageKey: 'img-3' }]
	};

	/** The step's document with its words, after `edit` ran through the plugin. */
	function after(edit: (state: EditorState) => Transaction): EditorState {
		const state = EditorState.create({
			doc: ProseMirrorNode.fromJSON(
				schema,
				textToDoc('Teig ins kochende Salzwasser schaben.', words)
			),
			plugins: [dropAbsent()]
		});
		return state.apply(edit(state));
	}

	/** Deletes the first occurrence of `text` from the document's only paragraph. */
	const remove = (text: string) => (state: EditorState) => {
		const from = state.doc.textContent.indexOf(text) + 1;
		return state.tr.delete(from, from + text.length);
	};

	it('drops a photo link whose phrase was edited away', () => {
		const state = after(remove(' schaben'));
		expect(wordsOf(state.doc).photos).toEqual([]);
		expect(wordsOf(state.doc).references).toEqual(words.references);
	});

	it('keeps a photo link whose phrase is still in the text', () => {
		const state = after(remove('Teig '));
		expect(wordsOf(state.doc).photos).toEqual(words.photos);
		expect(wordsOf(state.doc).references).toEqual([]);
	});
});
