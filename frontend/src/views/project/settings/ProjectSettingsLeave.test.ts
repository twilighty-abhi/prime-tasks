import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest'
import {mount, flushPromises, type VueWrapper} from '@vue/test-utils'
import {setActivePinia, createPinia} from 'pinia'
import {createI18n} from 'vue-i18n'
import {createRouter, createMemoryHistory} from 'vue-router'
import {QueryClient, VueQueryPlugin} from '@tanstack/vue-query'

import type {ProjectResponse} from '@/client/queries/projects'

const {projects, sdk} = vi.hoisted(() => ({
	projects: {} as Record<number, ProjectResponse>,
	sdk: {projectUsersList: vi.fn(), projectSharesLeave: vi.fn()},
}))

vi.mock('@/composables/useProjects', () => ({useProjects: () => ({projects})}))
vi.mock('@/client/generated', async importOriginal => ({...(await importOriginal<object>()), ...sdk}))
vi.mock('@/stores/auth', () => ({useAuthStore: () => ({info: {id: 2, username: 'me'}})}))

import ProjectSettingsLeave from '@/views/project/settings/ProjectSettingsLeave.vue'
import Modal from '@/components/misc/Modal.vue'
import testid from '@/directives/testid'
import enMessages from '@/i18n/lang/en.json'

const i18n = createI18n({legacy: false, locale: 'en', messages: {en: enMessages}})

async function mountModal() {
	const router = createRouter({
		history: createMemoryHistory(),
		routes: [
			{path: '/projects/:projectId/settings/leave', name: 'project.settings.leave', component: ProjectSettingsLeave},
			{path: '/teams/:id/edit', name: 'teams.edit', component: {template: '<div />'}},
			{path: '/projects/:projectId', name: 'project.index', component: {template: '<div />'}},
			{path: '/', name: 'home', component: {template: '<div />'}},
		],
	})
	await router.push('/projects/7/settings/leave')
	await router.isReady()
	const wrapper = mount(ProjectSettingsLeave, {
		global: {
			plugins: [i18n, router, [VueQueryPlugin, {queryClient: new QueryClient({defaultOptions: {queries: {retry: false}}})}]],
			components: {Modal},
			directives: {cy: testid},
			stubs: {
				BaseButton: {template: '<a v-bind="$attrs"><slot /></a>'},
				XButton: {props: ['to'], template: '<button type="button" :data-to="to ? JSON.stringify(to) : undefined" v-bind="$attrs"><slot /></button>'},
				Icon: true,
			},
		},
		attachTo: document.body,
	})
	await flushPromises()
	return wrapper
}

describe('ProjectSettingsLeave', () => {
	let wrapper: VueWrapper | undefined

	beforeEach(() => {
		setActivePinia(createPinia())
		vi.resetAllMocks()
		document.body.innerHTML = ''
		Object.keys(projects).forEach(id => delete projects[Number(id)])
	})

	afterEach(() => {
		wrapper?.unmount()
		wrapper = undefined
	})

	it('asks to confirm when you have a direct share', async () => {
		projects[7] = {id: 7, title: 'Website relaunch', shared_via: {type: 'direct', permission: 0}} as ProjectResponse
		sdk.projectUsersList.mockResolvedValue({data: {items: [{username: 'me'}], total_pages: 1}})
		wrapper = await mountModal()
		expect(document.body.textContent).toContain('Leave this project')
		expect(document.body.textContent).not.toContain('Your access comes from the team')
	})

	it('explains a team-only access and links to the team', async () => {
		projects[7] = {id: 7, title: 'Website relaunch', shared_via: {type: 'team', permission: 1, team_id: 5, team_name: 'Design'}} as ProjectResponse
		sdk.projectUsersList.mockResolvedValue({data: {items: [{username: 'someone'}], total_pages: 1}})
		wrapper = await mountModal()
		expect(document.body.textContent).toContain('Your access comes from the team Design. Leave the team or ask a team admin to remove you.')
		const link = document.body.querySelector('[data-to*="teams.edit"]')
		expect(link?.getAttribute('data-to')).toContain('"id":5')
		expect(sdk.projectSharesLeave).not.toHaveBeenCalled()
	})
})
