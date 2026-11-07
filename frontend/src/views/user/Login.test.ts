import {beforeEach, describe, it, expect, vi} from 'vitest'
import {mount, flushPromises} from '@vue/test-utils'
import {createI18n} from 'vue-i18n'
import {reactive} from 'vue'
import Login from './Login.vue'
import Button from '@/components/input/Button.vue'
import en from '@/i18n/lang/en.json'

const {configStore, authStore, redirectToProvider} = vi.hoisted(() => ({
	configStore: {
		fold_local_login: false,
		auth: {
			local: {enabled: true, registration_enabled: false},
			ldap: {enabled: false},
			openid_connect: {
				enabled: true,
				providers: [] as unknown[],
			},
		},
	},
	authStore: {
		authenticated: false,
		isLoading: false,
		needsTotpPasscode: false,
		verifyEmail: vi.fn(),
		login: vi.fn(),
		setNeedsTotpPasscode: vi.fn(),
	},
	redirectToProvider: vi.fn(),
}))

vi.mock('@/stores/config', () => ({useConfigStore: () => configStore}))
vi.mock('@/stores/auth', () => ({useAuthStore: () => authStore, JUST_LOGGED_OUT_KEY: 'justLoggedOut'}))
vi.mock('vue-router', () => ({
	useRoute: () => ({hash: '', query: {}}),
	useRouter: () => ({push: vi.fn()}),
}))
vi.mock('@/helpers/redirectToProvider', () => ({
	getAutoRedirectProvider: () => undefined,
	redirectToProvider,
}))
vi.mock('@/helpers/desktopAuth', () => ({isDesktopApp: () => false}))
vi.mock('@/composables/useRedirectToLastVisited', () => ({
	useRedirectToLastVisited: () => ({redirectIfSaved: vi.fn()}),
}))
vi.mock('@/composables/useTitle', () => ({useTitle: vi.fn()}))
vi.mock('@/message', () => ({getErrorText: (e: unknown) => String(e)}))

const i18n = createI18n({legacy: false, locale: 'en', messages: {en}})

const google = {
	key: 'google',
	name: 'Google',
	auth_url: 'https://accounts.google.com/o/oauth2/v2/auth',
	client_id: 'id.apps.googleusercontent.com',
	scope: 'openid email profile',
	logout_url: '',
}

function mountLogin() {
	return mount(Login, {
		attachTo: document.body,
		global: {
			plugins: [i18n],
			components: {XButton: Button},
			directives: {focus: {}},
			stubs: {
				RouterLink: true,
				Icon: true,
				Password: true,
				FormField: true,
				FormCheckbox: true,
				DesktopLogin: true,
			},
		},
	})
}

function providerButton(wrapper: ReturnType<typeof mountLogin>) {
	return wrapper.findAll('button').find(b => b.text().includes('Google'))
}

function otherWaysToggle(wrapper: ReturnType<typeof mountLogin>) {
	return wrapper.findAll('button').find(b => b.text() === 'Other ways to sign in')
}

describe('Login', () => {
	beforeEach(() => {
		vi.clearAllMocks()
		authStore.verifyEmail.mockResolvedValue(false)
		Object.assign(configStore, reactive({
			fold_local_login: false,
			auth: {
				local: {enabled: true, registration_enabled: false},
				ldap: {enabled: false},
				openid_connect: {enabled: true, providers: [google]},
			},
		}))
	})

	describe('with auth.local.foldonlogin off', () => {
		it('shows the login form first and the providers as secondary buttons below it', async () => {
			const wrapper = mountLogin()
			await flushPromises()

			const form = wrapper.find('#loginform')
			expect(form.exists()).toBe(true)
			const provider = providerButton(wrapper)!
			expect(provider.text()).toBe('Log in with Google')
			expect(provider.classes()).toContain('is-outlined')
			expect(provider.classes()).not.toContain('is-primary')
			// The form precedes the provider button in document order.
			expect(form.element.compareDocumentPosition(provider.element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
			expect(otherWaysToggle(wrapper)).toBeUndefined()
			wrapper.unmount()
		})
	})

	describe('with auth.local.foldonlogin on', () => {
		beforeEach(() => {
			configStore.fold_local_login = true
		})

		it('leads with the provider as the primary button and folds the form', async () => {
			const wrapper = mountLogin()
			await flushPromises()

			const provider = providerButton(wrapper)!
			expect(provider.text()).toBe('Sign in with Google')
			expect(provider.classes()).toContain('is-primary')
			expect(wrapper.find('#loginform').exists()).toBe(false)

			const toggle = otherWaysToggle(wrapper)!
			expect(toggle.attributes('aria-expanded')).toBe('false')
			expect(toggle.attributes('type')).toBe('button')
			wrapper.unmount()
		})

		it('starts the provider sign-in from the primary button', async () => {
			const wrapper = mountLogin()
			await flushPromises()

			await providerButton(wrapper)!.trigger('click')
			expect(redirectToProvider).toHaveBeenCalledWith(google)
			wrapper.unmount()
		})

		it('unfolds and folds the login form from the disclosure', async () => {
			const wrapper = mountLogin()
			await flushPromises()

			const toggle = otherWaysToggle(wrapper)!
			await toggle.trigger('click')
			expect(wrapper.find('#loginform').exists()).toBe(true)
			expect(toggle.attributes('aria-expanded')).toBe('true')
			expect(toggle.attributes('aria-controls')).toBe('loginform')

			await toggle.trigger('click')
			expect(wrapper.find('#loginform').exists()).toBe(false)
			expect(toggle.attributes('aria-expanded')).toBe('false')
			wrapper.unmount()
		})

		it('shows the form unfolded when no provider is configured', async () => {
			configStore.auth.openid_connect.providers = []
			const wrapper = mountLogin()
			await flushPromises()

			expect(wrapper.find('#loginform').exists()).toBe(true)
			expect(otherWaysToggle(wrapper)).toBeUndefined()
			wrapper.unmount()
		})

		it('shows the providers unfolded when local and LDAP login are off', async () => {
			configStore.auth.local.enabled = false
			const wrapper = mountLogin()
			await flushPromises()

			expect(wrapper.find('#loginform').exists()).toBe(false)
			expect(otherWaysToggle(wrapper)).toBeUndefined()
			expect(providerButton(wrapper)!.text()).toBe('Log in with Google')
			wrapper.unmount()
		})
	})
})
