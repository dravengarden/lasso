// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

const site = 'https://dravengarden.github.io';

export default defineConfig({
  site,
  base: '/lasso',
  integrations: [
    starlight({
      title: 'Lasso',
      description:
        'Agent-native monorepo workspace — rope repositories into one workspace for AI agents.',
      favicon: '/favicon.svg',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/dravengarden/lasso',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/dravengarden/lasso/edit/main/website/',
      },
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        {
          label: 'Start here',
          items: [
            { label: 'Introduction', slug: 'index' },
            { label: 'Quick start', slug: 'guides/quick-start' },
            { label: 'Install', slug: 'guides/install' },
          ],
        },
        {
          label: 'Guides',
          items: [
            { label: 'Initialize a workspace', slug: 'guides/init' },
            { label: 'Add and remove modules', slug: 'guides/modules' },
            { label: 'Register projects', slug: 'guides/projects' },
            { label: 'Work items', slug: 'guides/work-items' },
          ],
        },
        {
          label: 'Concepts',
          items: [
            { label: 'Overview', slug: 'concepts/overview' },
            { label: 'Modules', slug: 'concepts/modules' },
            { label: 'Agent runtimes', slug: 'concepts/agent-runtimes' },
            { label: 'Acceptance', slug: 'concepts/acceptance' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'CLI', slug: 'reference/cli' },
            { label: 'Requirements', slug: 'reference/requirements' },
            { label: 'Skills', slug: 'reference/skills' },
          ],
        },
      ],
    }),
  ],
});
