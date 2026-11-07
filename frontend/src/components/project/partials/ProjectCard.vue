<template>
	<!-- prime-tasks: Yuyu event-card style (colour band with a dot grid, initial tile, Bricolage title, chips) -->
	<div
		class="project-card"
		:class="{'is-archived-card': project.is_archived}"
	>
		<div
			class="project-card__band"
			:class="{
				'is-neutral': !accentColor,
				'has-background': Boolean(blurHashUrl || background),
			}"
			:style="{
				'background-color': accentColor || undefined,
				'background-image': blurHashUrl ? `url(${blurHashUrl})` : undefined,
			}"
		>
			<div
				class="project-background background-fade-in"
				:class="{'is-visible': background}"
				:style="{'background-image': background ? `url(${background})` : undefined}"
			/>
		</div>
		<span
			class="project-card__mark"
			aria-hidden="true"
		>
			<Icon
				v-if="project.id < -1"
				icon="filter"
			/>
			<template v-else>{{ initial }}</template>
		</span>
		<div class="project-card__body">
			<div
				v-if="isShared || project.is_archived"
				class="project-card__chips"
			>
				<span
					v-if="isShared"
					class="project-card__chip"
				>{{ $t('project.sharedBadge') }}</span>
				<span
					v-if="project.is_archived"
					class="project-card__chip"
				>{{ $t('project.archived') }}</span>
			</div>
			<div
				class="project-title"
				aria-hidden="true"
			>
				{{ title }}
			</div>
		</div>
		<BaseButton
			class="project-button"
			:aria-label="project.title"
			:title="textOnlyDescription"
			:to="{
				name: 'project.index',
				params: { projectId: project.id}
			}"
		/>
		<div class="project-actions">
			<BaseButton
				v-if="!project.is_archived && project.id > -1"
				class="favorite"
				:aria-label="project.is_favorite ? $t('project.unfavorite') : $t('project.favorite')"
				:class="{'is-favorite': project.is_favorite}"
				@click.prevent.stop="toggleProjectFavorite"
			>
				<Icon :icon="project.is_favorite ? 'star' : ['far', 'star']" />
			</BaseButton>
			<ProjectSettingsDropdown
				v-if="typeof project.max_permission === 'number' && project.max_permission > PERMISSIONS.READ"
				class="settings-dropdown"
				:project="project"
			/>
		</div>
	</div>
</template>

<script lang="ts" setup>
import {computed} from 'vue'
import {usePatchProjectFavoriteMutation, type ProjectResponse} from '@/client/queries/projects'

import BaseButton from '@/components/base/BaseButton.vue'
import ProjectSettingsDropdown from '@/components/project/ProjectSettingsDropdown.vue'
import {PERMISSIONS} from '@/constants/permissions'

import {useProjectBackground} from '@/composables/useProjectBackground'
import {getProjectTitle} from '@/helpers/getProjectTitle'
import {htmlToPreviewText} from '@/helpers/htmlToPreviewText'

const props = defineProps<{
	project: ProjectResponse,
}>()

const {background, blurHashUrl} = useProjectBackground(() => props.project)

const favoriteMutation = usePatchProjectFavoriteMutation()

async function toggleProjectFavorite() {
	if (props.project.id <= 0 || props.project.is_archived) {
		return
	}
	await favoriteMutation.mutateAsync({id: props.project.id, isFavorite: !props.project.is_favorite})
}

const textOnlyDescription = computed(() => htmlToPreviewText(props.project.description ?? ''))

const title = computed(() => getProjectTitle(props.project))

// The first character of the title (a whole emoji, not half of one), upper-cased.
const initial = computed(() => (Array.from(title.value.trim())[0] ?? '').toLocaleUpperCase())

// Only a valid #rgb / #rrggbb value reaches the inline style; anything else gets the neutral band.
const accentColor = computed(() => {
	const hex = (props.project.hex_color ?? '').trim().replace(/^#/, '')
	return /^([0-9a-f]{3}|[0-9a-f]{6})$/i.test(hex) ? `#${hex}` : ''
})

const isShared = computed(() => Boolean(props.project.shared_via))
</script>

<style lang="scss" scoped>
// prime-tasks: Yuyu event card (docs/DESIGN.md §4.7 in Yuyu): canvas, 1px border, r-xl, flat until hover.
.project-card {
	--project-card-padding: var(--yy-space-4, 1rem);
	--project-card-band: 72px;
	--project-card-mark: 44px;
	position: relative;
	display: flex;
	flex-direction: column;
	min-block-size: 100%;
	background: var(--yy-bg-canvas, var(--white));
	border: 1px solid var(--yy-border, var(--grey-200));
	border-radius: var(--yy-radius-xl, 20px);
	box-shadow: none;
	transition: box-shadow var(--yy-duration-hover, 180ms) var(--yy-ease, ease);

	&:hover,
	&:focus-within {
		box-shadow: var(--yy-shadow-float, var(--shadow-md));
	}
}

// Clip the band, not the card, so the settings dropdown can overflow the card.
.project-card__band {
	position: relative;
	flex-shrink: 0;
	block-size: var(--project-card-band);
	border-start-start-radius: calc(var(--yy-radius-xl, 20px) - 1px);
	border-start-end-radius: calc(var(--yy-radius-xl, 20px) - 1px);
	overflow: hidden;
	background-size: cover;
	background-position: center;

	// Yuyu's cover placeholder: the accent with a 22px grid of 18% white dots.
	&::before {
		content: '';
		position: absolute;
		inset: 0;
		background-image: radial-gradient(circle, rgb(255 255 255 / 18%) 2.5px, transparent 3px);
		background-size: 22px 22px;
		background-position: 0.5px 0.5px;
	}

	&.has-background::before {
		display: none;
	}

	&.is-neutral {
		background-color: var(--yy-inverse-surface, #18181b);
	}
}

:global(:root.dark) .project-card__band.is-neutral {
	background-color: var(--yy-bg-emphasis, #2a2a2e);
}

.project-background {
	position: absolute;
	inset: 0;
	background-size: cover;
	background-repeat: no-repeat;
	background-position: center;
}

// Like Yuyu's date block: a canvas tile overlapping the band's bottom edge.
.project-card__mark {
	position: absolute;
	inset-block-start: calc(var(--project-card-band) - var(--project-card-mark) / 2);
	inset-inline-start: var(--project-card-padding);
	display: flex;
	align-items: center;
	justify-content: center;
	inline-size: var(--project-card-mark);
	block-size: var(--project-card-mark);
	background: var(--yy-bg-canvas, var(--white));
	border: 1px solid var(--yy-border, var(--grey-200));
	border-radius: var(--yy-radius-md, 12px);
	color: var(--yy-text, var(--text));
	font-family: var(--yy-font-display, $vikunja-font);
	font-weight: 800;
	font-size: 1.375rem;
	line-height: 1;

	.icon,
	svg {
		font-size: 1rem;
		color: var(--yy-text-muted, var(--grey-500));
	}
}

.project-card__body {
	display: flex;
	flex: 1;
	flex-direction: column;
	gap: var(--yy-space-2, .5rem);
	padding: calc(var(--project-card-mark) / 2 + var(--yy-space-3, .75rem)) var(--project-card-padding) var(--project-card-padding);
	min-inline-size: 0;
}

.project-card__chips {
	display: flex;
	flex-wrap: wrap;
	gap: 6px;
}

.project-card__chip {
	border-radius: var(--yy-radius-full, 999px);
	padding: 2px 10px;
	background: var(--yy-bg-muted, var(--grey-100));
	color: var(--yy-text-secondary, var(--grey-700));
	font-size: .75rem;
	font-weight: 700;
	line-height: 1.4;
}

.project-title {
	color: var(--yy-text, var(--text));
	font-family: var(--yy-font-display, $vikunja-font);
	font-weight: 800;
	font-size: 1.125rem;
	line-height: 1.25;
	letter-spacing: -0.01em;
	overflow: hidden;
	overflow-wrap: anywhere;

	display: -webkit-box;
	-webkit-line-clamp: 2;
	-webkit-box-orient: vertical;
}

.is-archived-card {
	.project-card__band,
	.project-card__mark {
		opacity: .6;
	}
}

// The whole card is one link; the actions sit above it.
.project-button {
	position: absolute;
	inset: 0;
	z-index: 1;
	border-radius: inherit;
}

.project-actions {
	position: absolute;
	inset-block-start: var(--yy-space-3, .75rem);
	inset-inline-end: var(--yy-space-3, .75rem);
	z-index: 2;
	display: flex;
	align-items: center;
	gap: var(--yy-space-2, .5rem);
}

// Round canvas icon buttons, legible on any band colour or background image.
.favorite,
.settings-dropdown :deep(.dropdown-trigger) {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	inline-size: 32px;
	block-size: 32px;
	border-radius: var(--yy-radius-full, 999px);
	background: var(--yy-bg-canvas, var(--white));
	color: var(--yy-text, var(--text));
	transition: opacity var(--yy-duration-hover, 180ms) var(--yy-ease, ease), color var(--yy-duration-hover, 180ms) var(--yy-ease, ease);
}

.favorite,
.settings-dropdown {
	opacity: 1;

	&:hover {
		color: var(--warning);
	}
}

.favorite.is-favorite {
	color: var(--warning);
}

@media(hover: hover) and (pointer: fine) {
	.project-card .favorite:not(.is-favorite),
	.project-card .settings-dropdown {
		opacity: 0;
	}

	.project-card:hover .favorite,
	.project-card:hover .settings-dropdown,
	.project-card:focus-within .favorite,
	.project-card:focus-within .settings-dropdown,
	.settings-dropdown:has([aria-expanded="true"]) {
		opacity: 1;
	}
}

.background-fade-in {
	opacity: 0;
	transition: opacity $transition;
	transition-delay: $transition-duration * 2; // To fake an appearing background

	&.is-visible {
		opacity: 1;
	}
}
</style>
