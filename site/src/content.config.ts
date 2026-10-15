import { defineCollection } from 'astro:content';
import { docsLoader } from '@astrojs/starlight/loaders';
import { docsSchema } from '@astrojs/starlight/schema';
import { z } from 'astro/zod';

// src/content/docs is a symlink to ../wiki. Starlight derives sidebar
// paths from src/content/docs, so a custom glob base outside it breaks autogenerate.
export const collections = {
	docs: defineCollection({
		loader: docsLoader(),
		schema: docsSchema({ extend: z.object({ date: z.coerce.date().optional(), teaser: z.string().optional(), minRead: z.number().optional(), tags: z.array(z.string()).optional() }) }),
	}),
};
