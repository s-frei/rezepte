// Renders content/api/reference.mdx from public/openapi.json. Deterministic:
// tags, operations and schemas are sorted. Never edit the output by hand.
import { readFileSync, writeFileSync } from 'node:fs';

type Schema = {
	$ref?: string;
	type?: string | string[];
	format?: string;
	items?: Schema;
	properties?: Record<string, Schema>;
	required?: string[];
	description?: string;
	enum?: unknown[];
	minLength?: number;
	maxLength?: number;
	minimum?: number;
	maximum?: number;
	pattern?: string;
	minItems?: number;
	maxItems?: number;
};
type Parameter = { name: string; in: string; required?: boolean; description?: string; schema?: Schema };
type Content = Record<string, { schema?: Schema }>;
type SecurityScheme = { type: string; scheme?: string; in?: string; description?: string };
type Operation = {
	operationId: string;
	summary?: string;
	description?: string;
	tags?: string[];
	parameters?: Parameter[];
	requestBody?: { content?: Content };
	responses?: Record<string, { description?: string; content?: Content }>;
	security?: Record<string, string[]>[];
};
type Document = {
	info: { title: string; version: string };
	paths: Record<string, Record<string, Operation>>;
	components?: { schemas?: Record<string, Schema>; securitySchemes?: Record<string, SecurityScheme> };
};

const doc = JSON.parse(readFileSync('public/openapi.json', 'utf8')) as Document;
const schemas = doc.components?.schemas ?? {};
const securitySchemes = doc.components?.securitySchemes ?? {};

/** MDX treats {, } and < as expressions/JSX; tables need | escaped. */
const text = (s: string | undefined) =>
	(s ?? '').replace(/[{}]/g, (c) => `\\${c}`).replace(/</g, '&lt;').replace(/\|/g, '\\|').replace(/\s*\n\s*/g, ' ');
const refName = (ref: string) => ref.split('/').pop() ?? ref;
const anchor = (s: string) => s.toLowerCase().replace(/[^a-z0-9]+/g, '-');

function typeOf(s: Schema | undefined): string {
	if (!s) return '–';
	if (s.$ref) return `[${refName(s.$ref)}](#${anchor(refName(s.$ref))})`;
	const types = Array.isArray(s.type) ? s.type : [s.type ?? (s.items ? 'array' : 'object')];
	if (s.items) {
		const arr = `${typeOf(s.items)}[]`;
		return types.includes('null') ? `${arr} \\| null` : arr;
	}
	return types.map((t) => (t === 'string' && s.format ? `${t} (${s.format})` : t)).join(' \\| ');
}

function constraints(s: Schema): string {
	const c: string[] = [];
	if (s.enum) c.push(`one of ${s.enum.map((v) => `\`${String(v)}\``).join(', ')}`);
	if (s.minLength !== undefined || s.maxLength !== undefined) c.push(`length ${s.minLength ?? 0}–${s.maxLength ?? '∞'}`);
	if (s.minimum !== undefined || s.maximum !== undefined) c.push(`${s.minimum ?? '−∞'} to ${s.maximum ?? '∞'}`);
	if (s.minItems !== undefined || s.maxItems !== undefined) c.push(`${s.minItems ?? 0}–${s.maxItems ?? '∞'} items`);
	if (s.pattern) c.push(`pattern \`${text(s.pattern)}\``);
	return c.join(', ');
}

function propertyTable(s: Schema): string[] {
	if (!s.properties) return [];
	const required = new Set(s.required ?? []);
	return [
		'| Field | Type | Required | Notes |',
		'|---|---|---|---|',
		...Object.entries(s.properties)
			.sort(([a], [b]) => a.localeCompare(b))
			.map(
				([name, p]) =>
					`| \`${name}\` | ${typeOf(p)} | ${required.has(name) ? 'yes' : 'no'} | ${[text(p.description), constraints(p)].filter(Boolean).join('; ')} |`
			),
		''
	];
}

function bodySchema(content: Content | undefined): Schema | undefined {
	if (!content) return undefined;
	return content['application/json']?.schema ?? content['multipart/form-data']?.schema ?? content['application/problem+json']?.schema;
}

/** Joins items the way English prose does: "a", "a and b", "a, b and c". */
function englishList(items: string[]): string {
	if (items.length <= 1) return items.join('');
	if (items.length === 2) return `${items[0]} and ${items[1]}`;
	return `${items.slice(0, -1).join(', ')} and ${items[items.length - 1]}`;
}

/**
 * How a security scheme reads in prose, derived from its OpenAPI shape
 * (never from the scheme's name) so a scheme this generator has never seen
 * still renders, using its own description as the fallback noun.
 */
function schemeInfo(scheme: SecurityScheme): { noun: string; inline: string; plural: string } {
	if (scheme.type === 'apiKey' && scheme.in === 'cookie') return { noun: 'session cookie', inline: 'session cookie', plural: 'session cookies' };
	if (scheme.type === 'http' && scheme.scheme === 'bearer') return { noun: 'API token', inline: 'an API token', plural: 'API tokens' };
	const noun = scheme.description ? text(scheme.description) : scheme.type;
	return { noun, inline: `${/^[aeiou]/i.test(noun) ? 'an' : 'a'} ${noun}`, plural: `${noun}s` };
}

/** One sentence describing which credentials an operation accepts. */
function authSentence(op: Operation): string {
	if (!op.security?.length) return 'none';
	const used = new Set<string>();
	const alternatives = op.security.map((requirement) =>
		englishList(
			Object.entries(requirement).map(([name, scopes]) => {
				used.add(name);
				const info = schemeInfo(securitySchemes[name] ?? { type: name });
				return scopes.length ? `${info.inline} with ${englishList(scopes.map((s) => `\`${s}\``))}` : info.inline;
			})
		)
	);
	const sentence = alternatives.join(', or ');
	const excluded = Object.keys(securitySchemes).filter((name) => !used.has(name));
	if (!excluded.length) return sentence;
	const excludedLabel = englishList(excluded.map((name) => schemeInfo(securitySchemes[name]).plural));
	return `${sentence} only — ${excludedLabel} cannot use this operation`;
}

const byTag = new Map<string, { method: string; path: string; op: Operation }[]>();
for (const [p, methods] of Object.entries(doc.paths)) {
	for (const [m, op] of Object.entries(methods)) {
		const tag = op.tags?.[0] ?? 'other';
		byTag.set(tag, [...(byTag.get(tag) ?? []), { method: m.toUpperCase(), path: p, op }]);
	}
}

const out: string[] = [
	'---',
	'title: API reference',
	`description: Generated from the OpenAPI document of ${doc.info.title} ${doc.info.version}`,
	'---',
	'',
	'{/* Generated by scripts/generate-api.ts - do not edit. Regenerate: mise run //docs/user:openapi */}',
	'',
	'<Callout type="info">',
	'  Each operation below states which credentials it accepts. The source document is available as [openapi.json](/openapi.json) and rendered interactively by the running app at `/api/v1/docs`.',
	'</Callout>',
	''
];
for (const [tag, ops] of [...byTag.entries()].sort(([a], [b]) => a.localeCompare(b))) {
	out.push(`## ${tag}`, '');
	for (const { method, path, op } of ops.sort((a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method))) {
		out.push(`### ${method} ${text(path)}`, '');
		if (op.summary) out.push(text(op.summary), '');
		if (op.description) out.push(text(op.description), '');
		out.push(`**Authentication:** ${authSentence(op)}`, '');
		if (op.parameters?.length) {
			out.push('**Parameters**', '', '| Name | In | Type | Required | Description |', '|---|---|---|---|---|');
			for (const p of op.parameters) out.push(`| \`${p.name}\` | ${p.in} | ${typeOf(p.schema)} | ${p.required ? 'yes' : 'no'} | ${text(p.description)} |`);
			out.push('');
		}
		const body = bodySchema(op.requestBody?.content);
		if (body) out.push(`**Request body:** ${typeOf(body)}`, '', ...propertyTable(body));
		out.push('**Responses**', '', '| Status | Description | Body |', '|---|---|---|');
		for (const [code, r] of Object.entries(op.responses ?? {}).sort()) out.push(`| ${code} | ${text(r.description)} | ${typeOf(bodySchema(r.content))} |`);
		out.push('');
	}
}
out.push('## Authentication', '');
for (const [name, scheme] of Object.entries(securitySchemes).sort(([a], [b]) => a.localeCompare(b))) {
	const { noun } = schemeInfo(scheme);
	out.push(`### ${noun.charAt(0).toUpperCase()}${noun.slice(1)}`, '');
	if (scheme.description) out.push(text(scheme.description), '');
}
out.push('## Schemas', '');
for (const [name, s] of Object.entries(schemas).sort(([a], [b]) => a.localeCompare(b))) {
	out.push(`### ${name}`, '');
	if (s.description) out.push(text(s.description), '');
	out.push(...propertyTable(s));
}
writeFileSync('content/api/reference.mdx', out.join('\n'));
console.log(`reference.mdx: ${byTag.size} tags, ${Object.keys(schemas).length} schemas`);
