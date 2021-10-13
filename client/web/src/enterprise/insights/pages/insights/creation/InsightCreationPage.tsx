import React, { useContext, useMemo } from 'react'
import { useHistory } from 'react-router'

import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useObservable } from '@sourcegraph/shared/src/util/useObservable'

import { InsightsApiContext } from '../../../core/backend/api-provider'
import { InsightDashboard, isVirtualDashboard, Insight } from '../../../core/types'
import { isUserSubject } from '../../../core/types/subjects'
import { useQueryParameters } from '../../../hooks/use-query-parameters'

import { LangStatsInsightCreationPage } from './lang-stats/LangStatsInsightCreationPage'
import { SearchInsightCreationPage } from './search-insight/SearchInsightCreationPage'

export enum InsightCreationPageType {
    LangStats = 'lang-stats',
    Search = 'search-based',
}

const getVisibilityFromDashboard = (dashboard: InsightDashboard | null): string | undefined => {
    if (!dashboard || isVirtualDashboard(dashboard)) {
        return undefined
    }

    return dashboard.owner.id
}

interface InsightCreateEvent {
    subjectId: string
    insight: Insight
}

interface InsightCreationPageProps extends TelemetryProps {
    mode: InsightCreationPageType
}

export const InsightCreationPage: React.FunctionComponent<InsightCreationPageProps> = props => {
    const { mode, telemetryService } = props

    const history = useHistory()
    const { dashboardId } = useQueryParameters(['dashboardId'])
    const { getDashboardById, getInsightSubjects, createInsight } = useContext(InsightsApiContext)

    const dashboard = useObservable(useMemo(() => getDashboardById(dashboardId), [getDashboardById, dashboardId]))

    const subjects = useObservable(useMemo(() => getInsightSubjects(), [getInsightSubjects]))

    if (dashboard === undefined || subjects === undefined) {
        return <LoadingSpinner />
    }

    const handleInsightCreateRequest = async (event: InsightCreateEvent): Promise<void> =>
        createInsight({ ...event, dashboard }).toPromise()

    const handleInsightSuccessfulCreation = (insight: Insight): void => {
        if (!dashboard || isVirtualDashboard(dashboard)) {
            // Navigate to the dashboard page with new created dashboard
            history.push(`/insights/dashboards/${insight.visibility}`)

            return
        }

        if (dashboard.owner.id === insight.visibility) {
            history.push(`/insights/dashboards/${dashboard.id}`)
        } else {
            history.push(`/insights/dashboards/${insight.visibility}`)
        }
    }

    const handleCancel = (): void => {
        history.push(`/insights/dashboards/${dashboard?.id ?? 'all'}`)
    }

    // Calculate initial value for the visibility setting
    const personalVisibility = subjects.find(isUserSubject)?.id ?? ''
    const dashboardBasedVisibility = getVisibilityFromDashboard(dashboard)
    const insightVisibility = dashboardBasedVisibility ?? personalVisibility

    if (mode === InsightCreationPageType.Search) {
        return (
            <SearchInsightCreationPage
                subjects={subjects}
                visibility={insightVisibility}
                telemetryService={telemetryService}
                onInsightCreateRequest={handleInsightCreateRequest}
                onSuccessfulCreation={handleInsightSuccessfulCreation}
                onCancel={handleCancel}
            />
        )
    }

    return (
        <LangStatsInsightCreationPage
            subjects={subjects}
            visibility={insightVisibility}
            telemetryService={telemetryService}
            onInsightCreateRequest={handleInsightCreateRequest}
            onSuccessfulCreation={handleInsightSuccessfulCreation}
            onCancel={handleCancel}
        />
    )
}
