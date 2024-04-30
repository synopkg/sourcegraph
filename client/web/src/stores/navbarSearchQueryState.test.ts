import { describe, expect, it } from 'vitest'

import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import { SearchMode } from '@sourcegraph/shared/src/search'

import { parseSearchURL } from '../search'

import { setQueryStateFromSettings, setQueryStateFromURL, useNavbarQueryState } from './navbarSearchQueryState'

describe('navbar query state', () => {
    describe('set state from settings', () => {
        it('sets default search pattern', () => {
            setQueryStateFromSettings({
                subjects: [],
                final: {
                    'search.defaultPatternType': SearchPatternType.regexp,
                    experimentalFeatures: {
                        keywordSearch: false,
                    },
                },
            })

            expect(
                useNavbarQueryState.getState(),
                'got patternType' + useNavbarQueryState.getState().searchPatternType
            ).toHaveProperty('searchPatternType', SearchPatternType.regexp)
        })

        it('sets default case sensitivity', () => {
            setQueryStateFromSettings({
                subjects: [],
                final: {
                    'search.defaultCaseSensitive': true,
                },
            })

            expect(useNavbarQueryState.getState()).toHaveProperty('searchCaseSensitivity', true)
        })
    })

    describe('set state from URL', () => {
        it('sets the query from URL', () => {
            setQueryStateFromURL(parseSearchURL('q=test'))

            expect(useNavbarQueryState.getState().queryState).toHaveProperty('query', 'test')
        })

        it('prefers query parameter over parsed query', () => {
            setQueryStateFromURL(parseSearchURL('q=test'), 'testparameter')

            expect(useNavbarQueryState.getState().queryState).toHaveProperty('query', 'testparameter')
        })

        it('sets the search pattern from URL parameter', () => {
            setQueryStateFromURL(parseSearchURL('q=context:global+&patternType=regexp'))

            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.regexp)
        })

        it('sets the search pattern from filter', () => {
            setQueryStateFromURL(parseSearchURL('q=context:global+patterntype:regexp'))

            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.regexp)
        })

        it('should not set patternType if query is empty', () => {
            useNavbarQueryState.setState({ searchPatternType: SearchPatternType.standard })
            setQueryStateFromURL(parseSearchURL('q=patterntype:regexp'))

            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.standard)
        })

        it('sets case sensitivity from filter', () => {
            setQueryStateFromURL(parseSearchURL('q=context:global+case:yes'))

            expect(useNavbarQueryState.getState()).toHaveProperty('searchCaseSensitivity', true)
        })

        it('sets case sensitivity from URL parameter', () => {
            setQueryStateFromURL(parseSearchURL('q=context:global+&case=yes'))

            expect(useNavbarQueryState.getState()).toHaveProperty('searchCaseSensitivity', true)
        })

        it('should not update caseSensitive from filter if query is empty', () => {
            useNavbarQueryState.setState({ searchCaseSensitivity: false })
            setQueryStateFromURL(parseSearchURL('q=case:yes'))

            expect(useNavbarQueryState.getState().searchCaseSensitivity).toBe(false)
        })

        it('should not default to "standard" if patterntype is missing', () => {
            useNavbarQueryState.setState({ searchPatternType: SearchPatternType.keyword })
            setQueryStateFromURL(parseSearchURL('q=hello'))

            expect(useNavbarQueryState.getState().searchPatternType).toBe(SearchPatternType.keyword)
        })
    })

    describe('state initialization precedence', () => {
        // Note that the other tests already verify that user settings and URL
        // settings can override defaults

        it('prefers settings from URL over user settings', () => {
            setQueryStateFromSettings({
                subjects: [],
                final: {
                    'search.defaultPatternType': SearchPatternType.structural,
                },
            })
            setQueryStateFromURL(parseSearchURL('q=context:global+&patternType=regexp'))

            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.regexp)
        })

        it('prefers user settings over settings from empty URL', () => {
            setQueryStateFromURL(parseSearchURL(''))
            setQueryStateFromSettings({
                subjects: [],
                final: {
                    'search.defaultPatternType': SearchPatternType.structural,
                    experimentalFeatures: {
                        keywordSearch: false,
                    },
                },
            })

            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.structural)
        })

        it('does not prefer user settings over settings from URL with keyword search enabled', () => {
            setQueryStateFromURL(parseSearchURL('q=context:global+&patternType=standard'))
            setQueryStateFromSettings({
                subjects: [],
                final: {
                    experimentalFeatures: {
                        keywordSearch: true,
                    },
                },
            })

            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.standard)
        })

        it('does not prefer user settings over settings from URL with keyword search disabled', () => {
            setQueryStateFromURL(parseSearchURL('q=context:global+&patternType=regexp'))
            setQueryStateFromSettings({
                subjects: [],
                final: {
                    'search.defaultPatternType': SearchPatternType.structural,
                    experimentalFeatures: {
                        keywordSearch: false,
                    },
                },
            })

            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.regexp)
        })

        it('honors default user settings even if keyword search is enabled', () => {
            setQueryStateFromURL(parseSearchURL(''))
            setQueryStateFromSettings({
                subjects: [],
                final: {
                    'search.defaultPatternType': SearchPatternType.regexp,
                    experimentalFeatures: {
                        keywordSearch: true,
                    },
                },
            })

            expect(useNavbarQueryState.getState()).toHaveProperty('searchMode', SearchMode.Precise)
            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.regexp)
        })

        it('chooses correct defaults when keyword search is enabled', () => {
            setQueryStateFromURL(parseSearchURL(''))
            setQueryStateFromSettings({
                subjects: [],
                final: {
                    experimentalFeatures: {
                        keywordSearch: true,
                    },
                },
            })

            expect(useNavbarQueryState.getState()).toHaveProperty('searchMode', SearchMode.Precise)
            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.keyword)
        })

        it('chooses correct defaults when keyword search is not set', () => {
            setQueryStateFromURL(parseSearchURL(''))
            setQueryStateFromSettings({
                subjects: [],
                final: {},
            })

            expect(useNavbarQueryState.getState()).toHaveProperty('searchMode', SearchMode.Precise)
            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.keyword)
        })

        it('chooses correct defaults when keyword search is disabled', () => {
            setQueryStateFromURL(parseSearchURL(''))
            setQueryStateFromSettings({
                subjects: [],
                final: {
                    experimentalFeatures: {
                        keywordSearch: false,
                    },
                },
            })

            expect(useNavbarQueryState.getState()).toHaveProperty('searchMode', SearchMode.SmartSearch)
            expect(useNavbarQueryState.getState()).toHaveProperty('searchPatternType', SearchPatternType.standard)
        })
    })
})
