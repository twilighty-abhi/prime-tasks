import {test, expect} from '../../support/fixtures'
import {ProjectFactory} from '../../factories/project'
import {TaskFactory} from '../../factories/task'
import {UserProjectFactory} from '../../factories/users_project'

test('shows which bot holds a claim on the task until it is released', async ({authenticatedPage: page, apiContext, userToken}) => {
	const auth = {Authorization: `Bearer ${userToken}`}
	const projects = await ProjectFactory.create(1)
	const tasks = await TaskFactory.create(1, {
		project_id: projects[0].id,
		title: 'Claimed task',
	})

	const bot = await apiContext.post('/api/v2/user/bots', {
		headers: auth,
		data: {
			username: 'bot-claimer',
			name: 'Claim Bot',
		},
	})
	expect(bot.ok()).toBe(true)
	const botID = (await bot.json()).id
	await UserProjectFactory.create(1, {
		project_id: projects[0].id,
		user_id: botID,
		permission: 1,
	})
	const token = await apiContext.post('/api/v2/tokens', {
		headers: auth,
		data: {
			title: 'claim',
			owner_id: botID,
			permissions: {tasks: ['claim', 'release']},
			expires_at: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
		},
	})
	expect(token.ok()).toBe(true)
	const botAuth = {Authorization: `Bearer ${(await token.json()).token}`}

	const claim = await apiContext.post(`/api/v2/tasks/${tasks[0].id}/claim`, {headers: botAuth})
	expect(claim.ok()).toBe(true)

	await page.goto(`/tasks/${tasks[0].id}`)
	const notice = page.locator('[data-cy="taskClaimNotice"]')
	await expect(notice).toContainText('Claim Bot')
	await expect(notice).toContainText('until')
	await expect(page.locator('.task-view')).toContainText('agent: working')

	const release = await apiContext.post(`/api/v2/tasks/${tasks[0].id}/release`, {
		headers: botAuth,
		data: {comment: 'Done for now'},
	})
	expect(release.ok()).toBe(true)

	await page.reload()
	await expect(page.locator('.task-view .heading')).toBeVisible()
	await expect(notice).toHaveCount(0)
	await expect(page.locator('.task-view')).toContainText('Done for now')
})
