import {test, expect} from '../../support/fixtures'
import {UserFactory} from '../../factories/user'
import {TeamFactory} from '../../factories/team'
import {TeamMemberFactory} from '../../factories/team_member'
import {UserProjectFactory} from '../../factories/users_project'
import {login} from '../../support/authenticateUser'
import {createProjects} from '../project/prepareProjects'

// prime-tasks F5: people and teams are shared from one dialog.
for (const kind of ['user', 'team'] as const) {
	test(`adds, changes and removes a project ${kind} share`, async ({authenticatedPage: page}) => {
		const users = await UserFactory.create(2)
		await createProjects(1)
		const [team] = await TeamFactory.create(1, {id: 1, name: 'Sharing Team'})
		await TeamMemberFactory.create(1, {team_id: team.id, user_id: 1, admin: true})
		const name = kind === 'user' ? users[1].username : team.name
		await page.goto('/projects/1/settings/share')
		const dialog = page.locator('.share-dialog')
		await dialog.getByRole('combobox', {name: 'Search for people or teams'}).pressSequentially(name, {delay: 10})
		const option = dialog.getByRole('listbox').getByRole('option').filter({hasText: name})
		await expect(option).toBeVisible()
		const created = page.waitForResponse(r => new URL(r.url()).pathname === `/api/v2/projects/1/${kind}s` && r.request().method() === 'POST')
		await option.getByRole('button').click()
		expect((await created).ok()).toBe(true)

		const row = dialog.locator('.share-dialog__row').filter({hasText: name})
		await expect(row).toBeVisible()
		await expect(row).toContainText('Direct')
		const permission = row.getByRole('combobox', {name: `Permission for ${name}`})
		// New shares get the role picked next to the search, read & write by default.
		await expect(permission).toHaveValue('1')
		await permission.selectOption('2')
		await expect(permission).toHaveValue('2')
		await page.reload()
		await expect(permission).toHaveValue('2')

		await row.getByRole('button', {name: `Remove ${name}`}).click()
		await row.getByRole('button', {name: 'Cancel', exact: true}).click()
		await expect(row.getByRole('button', {name: `Remove ${name}`})).toBeVisible()
		await row.getByRole('button', {name: `Remove ${name}`}).click()
		await row.getByRole('button', {name: 'Remove', exact: true}).click()
		await expect(row).toHaveCount(0)
		await page.reload()
		await expect(dialog.locator('.share-dialog__row').first()).toBeVisible()
		await expect(row).toHaveCount(0)
	})
}

for (const kind of ['user', 'team'] as const) {
	test(`clears ${kind} share search results when the search is cleared`, async ({authenticatedPage: page}) => {
		const users = await UserFactory.create(2)
		await createProjects(1)
		const [team] = await TeamFactory.create(1, {id: 1, name: 'Sharing Team'})
		await TeamMemberFactory.create(1, {team_id: team.id, user_id: 1, admin: true})
		const name = kind === 'user' ? users[1].username : team.name
		await page.goto('/projects/1/settings/share')
		const dialog = page.locator('.share-dialog')
		const search = dialog.getByRole('combobox', {name: 'Search for people or teams'})
		await search.pressSequentially(name, {delay: 10})
		await expect(dialog.getByRole('listbox').getByRole('option').filter({hasText: name})).toBeVisible()
		await search.press('ControlOrMeta+a')
		await search.press('Backspace')
		await expect(dialog.getByRole('option')).toHaveCount(0)
	})
}

test('read-only members cannot manage shares', async ({page, apiContext}) => {
	const [, reader] = await UserFactory.create(2)
	await createProjects(1)
	await UserProjectFactory.create(1, {project_id: 1, user_id: reader.id, permission: 0})
	await login(page, apiContext, reader)
	await page.goto('/projects/1/settings/share')
	const dialog = page.locator('.share-dialog')
	const row = dialog.locator('.share-dialog__row').filter({hasText: reader.username})
	await expect(row).toContainText('You')
	await expect(dialog.getByRole('combobox', {name: 'Search for people or teams'})).toHaveCount(0)
	await expect(dialog.getByRole('button', {name: /^Remove/})).toHaveCount(0)
	await expect(row.getByRole('combobox')).toHaveCount(0)
})
