import type {Page} from '@playwright/test'
import {test, expect} from '../../support/fixtures'
import {UserFactory} from '../../factories/user'
import {setupApiUrl} from '../../support/authenticateUser'
import {TEST_PASSWORD} from '../../support/constants'

// prime-tasks G1: with auth.local.foldonlogin on and an OpenID provider configured, the login
// page leads with the provider and folds the username/password form. The e2e API runs with the
// flag off, so the instance info is rewritten here; the provider itself is Dex, as in CI.
async function foldLocalLogin(page: Page) {
	await page.route(/\/api\/v2\/info$/, async route => {
		const response = await route.fetch()
		const info = await response.json()
		info.fold_local_login = true
		// Keep the spec independent of whether Dex is configured for this run.
		if (!info.auth.openid_connect.enabled || !info.auth.openid_connect.providers?.length) {
			info.auth.openid_connect = {
				enabled: true,
				providers: [{
					key: 'google',
					name: 'Google',
					auth_url: 'http://127.0.0.1:1/auth',
					client_id: 'client',
					scope: 'openid email profile',
					logout_url: '',
				}],
			}
		}
		await route.fulfill({response, json: info})
	})
}

test.describe('Login with the local form folded', () => {
	test.beforeEach(async ({page}) => {
		await setupApiUrl(page)
		await foldLocalLogin(page)
		await UserFactory.create(1, {username: 'test'})
	})

	test('leads with the provider and hides the password form', async ({page}) => {
		await page.goto('/login')

		const provider = page.getByRole('button', {name: /^Sign in with /})
		await expect(provider.first()).toBeVisible()
		await expect(provider.first()).toHaveClass(/is-primary/)
		await expect(page.locator('#username')).toHaveCount(0)
		await expect(page.getByRole('button', {name: 'Other ways to sign in'})).toHaveAttribute('aria-expanded', 'false')
	})

	test('unfolds the form from the keyboard and logs in with a password', async ({page}) => {
		await page.goto('/login')

		const toggle = page.getByRole('button', {name: 'Other ways to sign in'})
		await toggle.focus()
		await page.keyboard.press('Enter')
		await expect(toggle).toHaveAttribute('aria-expanded', 'true')

		await page.locator('input[id=username]').fill('test')
		await page.locator('input[id=password]').fill(TEST_PASSWORD)
		await page.locator('.button').filter({hasText: 'Login'}).click()
		await expect(page).toHaveURL('/')
		await expect(page.locator('main h1')).toContainText('test')
	})
})
