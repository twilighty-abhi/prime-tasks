import {afterEach, describe, expect, it, vi} from 'vitest'
import {ref} from 'vue'
import {enableAutoUnmount, mount, RouterLinkStub} from '@vue/test-utils'

import type {Label} from '@/client/generated'

const labels = ref<Label[]>([])

vi.mock('@/composables/useLabels', () => ({useLabels: () => ({labels, isPending: ref(false)})}))
vi.mock('@/stores/auth', () => ({useAuthStore: () => ({info: {id: 1}})}))
vi.mock('@/client/queries/labels', () => ({
	useUpdateLabelMutation: () => ({mutateAsync: vi.fn(), isPending: ref(false)}),
	useDeleteLabelMutation: () => ({mutateAsync: vi.fn(), isPending: ref(false)}),
}))
vi.mock('@/composables/useTitle', () => ({useTitle: vi.fn()}))
vi.mock('@/components/input/AsyncEditor', () => ({default: {render: () => null}}))
vi.mock('vue-i18n', async importOriginal => ({
	...await importOriginal<typeof import('vue-i18n')>(),
	useI18n: () => ({t: (key: string) => key}),
}))

import ListLabels from './ListLabels.vue'

enableAutoUnmount(afterEach)

function label(overrides: Partial<Label> = {}): Label {
	return {
		id: 1,
		title: 'Content',
		description: '',
		hex_color: '4f46e5',
		created_by: {id: 1, username: 'devadmin'},
		...overrides,
	} as Label
}

function mountLabels(list: Label[]) {
	labels.value = list
	return mount(ListLabels, {
		global: {
			stubs: {
				RouterLink: RouterLinkStub,
				XButton: {template: '<button class="x-button-stub"><slot /></button>'},
				BaseButton: {template: '<button class="base-button-stub" @click="$emit(\'click\', $event)"><slot /></button>'},
				Card: {props: ['title'], template: '<section class="card-stub" :data-title="title"><slot /></section>'},
				FormField: {template: '<div><slot /></div>'},
				ColorPicker: true,
				Modal: true,
				Icon: true,
			},
			mocks: {$t: (key: string) => key},
		},
	})
}

// prime-tasks: the labels page is a grid of Yuyu label cards (swatch, name, muted description)
// instead of floating pills.
describe('ListLabels', () => {
	it('renders each label as a card in the labels grid', () => {
		const wrapper = mountLabels([
			label(),
			label({id: 2, title: 'Design', hex_color: 'db2777'}),
		])

		const cards = wrapper.findAll('.labels-list .label-card')
		expect(cards).toHaveLength(2)
		expect(cards[0]!.find('.label-card__name').text()).toBe('Content')
		expect(cards[1]!.find('.label-card__name').text()).toBe('Design')
		// No more pill tags.
		expect(wrapper.find('.labels-list .tag').exists()).toBe(false)
	})

	it('shows the label colour as a swatch', () => {
		const wrapper = mountLabels([label({hex_color: 'db2777'})])

		const swatch = wrapper.find('.label-card__swatch')
		expect(swatch.exists()).toBe(true)
		expect(swatch.attributes('style')).toContain('background-color: #db2777')
		expect(swatch.attributes('aria-hidden')).toBe('true')
	})

	it('falls back to a neutral swatch when the label has no colour', () => {
		const wrapper = mountLabels([label({hex_color: ''})])

		expect(wrapper.find('.label-card__swatch').classes()).toContain('is-neutral')
	})

	it('shows the description as muted plain text, and nothing when it is empty', () => {
		const wrapper = mountLabels([
			label({description: '<p>Blog posts and <strong>copy</strong></p>'}),
			label({id: 2, title: 'Design', description: ''}),
		])

		const cards = wrapper.findAll('.label-card')
		expect(cards[0]!.find('.label-card__description').text()).toBe('Blog posts and copy')
		expect(cards[1]!.find('.label-card__description').exists()).toBe(false)
	})

	it('links the card to the tasks with that label', () => {
		const wrapper = mountLabels([label({id: 3})])

		const link = wrapper.findComponent(RouterLinkStub)
		expect(link.props('to')).toEqual({name: 'home', query: {labels: 3}})
		expect(link.classes()).toContain('label-card__link')
		expect(link.text()).toBe('Content')
	})

	it('offers an edit button only on labels the user created', async () => {
		const wrapper = mountLabels([
			label(),
			label({id: 2, title: 'Shared', created_by: {id: 7, username: 'priya'} as Label['created_by']}),
		])

		const cards = wrapper.findAll('.label-card')
		expect(cards[0]!.find('.label-edit-button').exists()).toBe(true)
		expect(cards[1]!.find('.label-edit-button').exists()).toBe(false)

		await cards[0]!.find('.label-edit-button').trigger('click')
		expect(wrapper.find('.card-stub[data-title="label.edit.header"]').exists()).toBe(true)
		expect(cards[0]!.classes()).toContain('is-editing')
	})

	it('keeps the explainer as a muted caption', () => {
		const wrapper = mountLabels([label()])

		const caption = wrapper.find('.labels-caption')
		expect(caption.exists()).toBe(true)
		expect(caption.text()).toBe('label.description')
	})
})
