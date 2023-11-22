import { at } from 'lodash'

import { type GitCommitFields, RepositoryType } from '../graphql-operations'

import { CodeHostType, FileNameOrExtension } from './constants'

export const isPerforceChangelistMappingEnabled = (): boolean =>
    window.context.experimentalFeatures.perforceChangelistMapping === 'enabled'

export const isPerforceDepotSource = (sourceType: string): boolean => sourceType === RepositoryType.PERFORCE_DEPOT

export const getRefType = (sourceType: RepositoryType | string): string =>
    isPerforceDepotSource(sourceType) ? 'changelist' : 'commit'

export const getCanonicalURL = (sourceType: RepositoryType | string, node: GitCommitFields): string =>
    isPerforceChangelistMappingEnabled() && isPerforceDepotSource(sourceType) && node.perforceChangelist
        ? node.perforceChangelist.canonicalURL
        : node.canonicalURL

export const getInitialSearchTerm = (repo: string): string => {
    const r = repo.split('/')
    return r.at(-1)?.trim() ?? ''
}

export const stringToCodeHostType = (codeHostType: string): CodeHostType => {
    switch (codeHostType) {
        case 'github': {
            return CodeHostType.GITHUB
        }
        case 'gitlab': {
            return CodeHostType.GITLAB
        }
        case 'bitbucketCloud': {
            return CodeHostType.BITBUCKETCLOUD
        }
        case 'gitolite': {
            return CodeHostType.GITOLITE
        }
        case 'awsCodeCommit': {
            return CodeHostType.AWSCODECOMMIT
        }
        case 'azureDevOps': {
            return CodeHostType.AZUREDEVOPS
        }
        default: {
            return CodeHostType.OTHER
        }
    }
}

export const contains = (arr: string[], target: string): boolean => {
    for (const i of arr) {
        if (i === target) {
            return true
        }
    }
    return false
}

export const getExtension = (
    file: string
): { name: FileNameOrExtension; extension: FileNameOrExtension; isTest: boolean } => {
    const e = {
        name: file as FileNameOrExtension,
        extension: file.split('.').pop() as FileNameOrExtension,
        isTest:
            file.endsWith('.test.js') ||
            file.endsWith('.test.jsx') ||
            file.endsWith('.test.ts') ||
            file.endsWith('.test.tsx') ||
            file.endsWith('_test.go'),
    }
    return e
}
