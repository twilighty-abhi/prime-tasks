<template>
	<div
		class="content loader-container is-max-width-desktop"
		:class="{ 'is-loading': loading}"
	>
		<XButton
			:to="{name:'teams.create'}"
			class="is-pulled-end"
			icon="plus"
		>
			{{ $t('team.create.title') }}
		</XButton>

		<h1>{{ $t('team.title') }}</h1>
		<!-- prime-tasks: Yuyu team cards (initial tile, Bricolage name, member meta) instead of a text list -->
		<ul
			v-if="teams.length > 0"
			class="teams"
		>
			<li
				v-for="team in teams"
				:key="team.id"
				class="team-card"
			>
				<span
					class="team-card__mark"
					aria-hidden="true"
				>{{ initial(team.name) }}</span>
				<div class="team-card__body">
					<RouterLink
						:to="{name: 'teams.edit', params: {id: team.id}}"
						class="team-card__link"
					>
						<span class="team-card__name">{{ team.name }}</span>
					</RouterLink>
					<p
						v-if="team.members || teamDescription(team.description)"
						class="team-card__meta"
					>
						<span
							v-if="team.members"
							class="team-card__members"
						>{{ $t('team.memberCount', {count: team.members.length}, team.members.length) }}</span>
						<span
							v-if="teamDescription(team.description)"
							class="team-card__description"
						>{{ teamDescription(team.description) }}</span>
					</p>
				</div>
			</li>
		</ul>
		<p
			v-else-if="!isFetching"
			class="has-text-centered has-text-grey is-italic"
		>
			{{ $t('team.noTeams') }}
			<RouterLink :to="{name: 'teams.create'}">
				{{ $t('team.create.title') }}.
			</RouterLink>
		</p>
		<Pagination
			:total-pages="totalPages"
			:current-page="page"
		/>
	</div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import {useRouteQuery} from '@vueuse/router'

import Pagination from '@/components/misc/Pagination.vue'
import {normalizePageNumber} from '@/client/queries/pagination'
import {useClampedPage} from '@/composables/useClampedPage'
import {useTeamsPage} from '@/composables/useTeams'
import { useTitle } from '@/composables/useTitle'
import {useDelayedLoading} from '@/composables/useDelayedLoading'
import {htmlToPreviewText} from '@/helpers/htmlToPreviewText'

const { t } = useI18n({useScope: 'global'})
useTitle(() => t('team.title'))

const page = useRouteQuery('page', '1', {transform: normalizePageNumber})
const teamsPage = useTeamsPage(page)
const {teams, totalPages, isFetching} = teamsPage
useClampedPage(page, teamsPage)
const loading = useDelayedLoading(isFetching)

// The first character of the name (a whole emoji, not half of one), upper-cased.
function initial(name?: string) {
	return (Array.from((name ?? '').trim())[0] ?? '').toLocaleUpperCase()
}

function teamDescription(description?: string) {
	return htmlToPreviewText(description ?? '')
}
</script>

<style lang="scss" scoped>
// prime-tasks: Yuyu team cards (docs/DESIGN.md §4.7 in Yuyu): canvas, 1px border, r-lg, flat until hover.
ul.teams {
	--team-card-mark: 44px;
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
	gap: var(--yy-space-3, .75rem);
	margin: 0;
	padding: 0;
	list-style: none;
}

.content ul.teams li + li {
	margin-block-start: 0;
}

.team-card {
	position: relative;
	display: flex;
	align-items: center;
	gap: var(--yy-space-3, .75rem);
	margin: 0;
	padding: var(--yy-space-4, 1rem);
	background: var(--yy-bg-canvas, var(--white));
	border: 1px solid var(--yy-border, var(--grey-200));
	border-radius: var(--yy-radius-lg, 16px);
	box-shadow: none;
	transition: box-shadow var(--yy-duration-hover, 180ms) var(--yy-ease, ease);

	&:hover,
	&:focus-within {
		box-shadow: var(--yy-shadow-float, var(--shadow-md));
	}
}

// Like the project card's initial tile.
.team-card__mark {
	display: flex;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	inline-size: var(--team-card-mark);
	block-size: var(--team-card-mark);
	background: var(--yy-bg-muted, var(--grey-100));
	border: 1px solid var(--yy-border, var(--grey-200));
	border-radius: var(--yy-radius-md, 12px);
	color: var(--yy-text, var(--text));
	font-family: var(--yy-font-display, $vikunja-font);
	font-weight: 800;
	font-size: 1.25rem;
	line-height: 1;
}

.team-card__body {
	display: flex;
	flex-direction: column;
	gap: 2px;
	min-inline-size: 0;
}

// The whole card is the link (stretched), but its accessible name stays the team name.
.team-card__link {
	color: var(--yy-text, var(--text));

	&::after {
		content: '';
		position: absolute;
		inset: 0;
		border-radius: inherit;
	}

	&:focus-visible {
		outline: none;

		&::after {
			outline: 2px solid var(--yy-text, var(--primary));
			outline-offset: 2px;
		}
	}
}

.team-card__name {
	font-family: var(--yy-font-display, $vikunja-font);
	font-weight: 800;
	font-size: 1.125rem;
	line-height: 1.25;
	letter-spacing: -0.01em;
	overflow-wrap: anywhere;
}

.team-card__meta {
	display: flex;
	flex-wrap: wrap;
	gap: 0 var(--yy-space-2, .5rem);
	margin: 0;
	color: var(--yy-text-muted, var(--grey-500));
	font-size: .8125rem;
	line-height: 1.45;
}

.team-card__description {
	overflow: hidden;
	overflow-wrap: anywhere;

	display: -webkit-box;
	-webkit-line-clamp: 2;
	-webkit-box-orient: vertical;

	.team-card__members + & {
		&::before {
			content: '· ';
		}
	}
}
</style>
