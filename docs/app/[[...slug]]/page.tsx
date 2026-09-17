import { DocsBody, DocsDescription, DocsPage, DocsTitle, MarkdownCopyButton, ViewOptionsPopover } from 'fumadocs-ui/layouts/docs/page';
import { createRelativeLink } from 'fumadocs-ui/mdx';
import { Card, Cards } from 'fumadocs-ui/components/card';
import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { getMDXComponents } from '@/components/mdx';
import { source } from '@/lib/source';
import { getPageMarkdownUrl } from '@/lib/shared';

type Props = { params: Promise<{ slug?: string[] }> };

// Static export cannot redirect at request time, so `/` renders a landing page.
function Landing() {
	return (
		<DocsPage>
			<DocsTitle>Rezepte Docs</DocsTitle>
			<DocsDescription>Pick a section.</DocsDescription>
			<DocsBody>
				<Cards>
					<Card href="/memory" title="Memory" description="Decisions, architecture and how-tos for coding agents." />
					<Card href="/user" title="User" description="End-user documentation." />
				</Cards>
			</DocsBody>
		</DocsPage>
	);
}

export default async function Page({ params }: Props) {
	const { slug } = await params;
	if (!slug || slug.length === 0) return <Landing />;
	const page = source.getPage(slug);
	if (!page) notFound();

	const MDX = page.data.body;
	return (
		<DocsPage toc={page.data.toc} full={page.data.full}>
			<DocsTitle>{page.data.title}</DocsTitle>
			<DocsDescription>{page.data.description}</DocsDescription>
			<div className="flex flex-row items-center gap-2 border-b pb-6">
				<MarkdownCopyButton markdownUrl={getPageMarkdownUrl(page)} />
				<ViewOptionsPopover
					markdownUrl={getPageMarkdownUrl(page)}
					githubUrl={`https://github.com/s-frei/rezepte/blob/develop/docs/${page.path}`}
				/>
			</div>
			<DocsBody>
				<MDX components={getMDXComponents({ a: createRelativeLink(source, page) })} />
			</DocsBody>
		</DocsPage>
	);
}

export function generateStaticParams() {
	return [{ slug: [] }, ...source.generateParams()];
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { slug } = await params;
	if (!slug || slug.length === 0) return { title: 'Rezepte Docs' };
	const page = source.getPage(slug);
	if (!page) notFound();
	return { title: page.data.title, description: page.data.description };
}
