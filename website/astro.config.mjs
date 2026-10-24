// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	site: 'https://vardius.github.io',
	base: '/shutdown',
	trailingSlash: 'always',
	// Pages import the runnable programs in ../examples, so the dev server may read outside this folder.
	vite: { server: { fs: { allow: ['..'] } } },
	integrations: [
		starlight({
			title: 'shutdown',
			description:
				'Graceful shutdown for Go: on SIGINT, SIGTERM or SIGHUP, run your stop functions with a deadline and get their errors back.',
			logo: { src: './src/assets/mark.svg', alt: '' },
			favicon: '/favicon.svg',
			components: { Hero: './src/components/Hero.astro' },
			// Long lines wrap instead of scrolling under the copy button on phones.
			expressiveCode: { defaultProps: { wrap: true } },
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/vardius/shutdown' }],
			customCss: ['./src/styles/custom.css'],
			sidebar: [
				{
					label: 'Start here',
					items: ['getting-started'],
				},
				{
					label: 'Guides',
					items: ['signals', 'stops'],
				},
				{
					label: 'Reference',
					items: [
						'migrating-from-v1',
						{ label: 'API reference', link: 'https://pkg.go.dev/github.com/vardius/shutdown/v2' },
					],
				},
			],
		}),
	],
});
