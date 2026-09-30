import defaultMdxComponents from 'fumadocs-ui/mdx';
import { Accordion, Accordions } from 'fumadocs-ui/components/accordion';
import { Callout } from 'fumadocs-ui/components/callout';
import { Card, Cards } from 'fumadocs-ui/components/card';
import { File, Files, Folder } from 'fumadocs-ui/components/files';
import { Step, Steps } from 'fumadocs-ui/components/steps';
import { Tab, Tabs } from 'fumadocs-ui/components/tabs';
import { TypeTable } from 'fumadocs-ui/components/type-table';
import type { MDXComponents } from 'mdx/types';
import { Mermaid } from '@/components/mdx/mermaid';
import { Hero } from '@/components/hero';
import { Screenshot } from '@/components/screenshot';
import { Audience, AudienceLegend } from '@/components/audience';

export function getMDXComponents(components?: MDXComponents) {
	return {
		...defaultMdxComponents,
		Accordion,
		Accordions,
		Audience,
		AudienceLegend,
		Callout,
		Card,
		Cards,
		File,
		Files,
		Folder,
		Hero,
		Mermaid,
		Screenshot,
		Step,
		Steps,
		Tab,
		Tabs,
		TypeTable,
		...components
	} satisfies MDXComponents;
}

export const useMDXComponents = getMDXComponents;
