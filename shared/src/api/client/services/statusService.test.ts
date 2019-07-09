import { StatusCompletion, CheckScope, StatusResult, Range } from '@sourcegraph/extension-api-classes'
import * as sourcegraph from 'sourcegraph'
import { of, throwError, Unsubscribable, combineLatest, from } from 'rxjs'
import { TestScheduler } from 'rxjs/testing'
import { createStatusService, WrappedStatus } from './statusService'
import { DiagnosticsService } from './diagnosticsService'
import { DiagnosticSeverity } from '../../types/diagnosticCollection'
import { TransferableStatus } from '../../types/status'
import { map, switchMap } from 'rxjs/operators'

const scheduler = () => new TestScheduler((a, b) => expect(a).toEqual(b))

const STATUS_1: WrappedStatus = {
    name: '',
    status: { title: '1', state: { result: StatusResult.Success, completion: StatusCompletion.Completed } },
}

const STATUS_2: WrappedStatus = { name: '', status: { title: '2', state: { completion: StatusCompletion.InProgress } } }

const SCOPE = CheckScope.Global

const EMPTY_DIAGNOSTICS: Pick<DiagnosticsService, 'observe'> = {
    observe: () => of([]),
}

describe('StatusService', () => {
    describe('observeStatuses', () => {
        test('no providers yields empty array', () =>
            scheduler().run(({ expectObservable }) =>
                expectObservable(createStatusService(EMPTY_DIAGNOSTICS, false).observeStatuses(SCOPE)).toBe('a', {
                    a: [],
                })
            ))

        test('single provider', () => {
            scheduler().run(({ cold, expectObservable }) => {
                const service = createStatusService(EMPTY_DIAGNOSTICS, false)
                service.registerStatusProvider('', {
                    provideStatus: () =>
                        cold<TransferableStatus | null>('abcd', {
                            a: null,
                            b: STATUS_1.status,
                            c: STATUS_2.status,
                            d: null,
                        }),
                })
                expectObservable(service.observeStatuses(SCOPE)).toBe('abcd', {
                    a: [],
                    b: [STATUS_1],
                    c: [STATUS_2],
                    d: [],
                })
            })
        })

        test('includes diagnostics', () => {
            const DIAGNOSTICS: [URL, sourcegraph.Diagnostic[]][] = [
                [
                    new URL('https://example.com'),
                    [{ message: 'm', range: new Range(1, 2, 3, 4), severity: DiagnosticSeverity.Error }],
                ],
            ]
            scheduler().run(({ cold, expectObservable }) => {
                const service = createStatusService(
                    {
                        observe: name => {
                            expect(name).toBe('d')
                            return of(DIAGNOSTICS)
                        },
                    },
                    false
                )
                service.registerStatusProvider('', {
                    provideStatus: () =>
                        cold<TransferableStatus>('a', {
                            a: { ...STATUS_1.status, _diagnosticCollectionName: 'd' },
                        }),
                })
                expectObservable(
                    service.observeStatuses(SCOPE).pipe(
                        switchMap(statuses =>
                            combineLatest(
                                statuses.map(status =>
                                    from(status.status.diagnostics || of([])).pipe(
                                        map(diagnostics => ({
                                            ...status,
                                            status: {
                                                ...status.status,
                                                _diagnosticCollectionName: undefined,
                                                diagnostics,
                                            },
                                        }))
                                    )
                                )
                            )
                        )
                    )
                ).toBe('a', {
                    a: [{ ...STATUS_1, status: { ...STATUS_1.status, diagnostics: DIAGNOSTICS } }],
                })
            })
        })

        test('merges results from multiple providers', () => {
            scheduler().run(({ cold, expectObservable }) => {
                const service = createStatusService(EMPTY_DIAGNOSTICS, false)
                const unsub1 = service.registerStatusProvider('1', {
                    provideStatus: () => of(STATUS_1.status),
                })
                let unsub2: Unsubscribable
                cold('-bc', {
                    b: () => {
                        unsub2 = service.registerStatusProvider('2', {
                            provideStatus: () => of(STATUS_2.status),
                        })
                    },
                    c: () => {
                        unsub1.unsubscribe()
                        unsub2.unsubscribe()
                    },
                }).subscribe(f => f())
                expectObservable(service.observeStatuses(SCOPE)).toBe('ab(cd)', {
                    a: [{ ...STATUS_1, name: '1' }],
                    b: [{ ...STATUS_1, name: '1' }, { ...STATUS_2, name: '2' }],
                    c: [{ ...STATUS_2, name: '2' }],
                    d: [],
                })
            })
        })

        test('suppresses errors', () => {
            scheduler().run(({ expectObservable }) => {
                const service = createStatusService(EMPTY_DIAGNOSTICS, false)
                service.registerStatusProvider('a', {
                    provideStatus: () => throwError(new Error('x')),
                })
                expectObservable(service.observeStatuses(SCOPE)).toBe('a', {
                    a: [],
                })
            })
        })
    })

    describe('observeStatus', () => {
        test('no providers yields null', () =>
            scheduler().run(({ expectObservable }) =>
                expectObservable(createStatusService(EMPTY_DIAGNOSTICS, false).observeStatus('', SCOPE)).toBe('a', {
                    a: null,
                })
            ))

        test('single provider', () => {
            scheduler().run(({ cold, expectObservable }) => {
                const service = createStatusService(EMPTY_DIAGNOSTICS, false)
                service.registerStatusProvider('', {
                    provideStatus: () =>
                        cold<TransferableStatus | null>('abcd', {
                            a: null,
                            b: STATUS_1.status,
                            c: STATUS_2.status,
                            d: null,
                        }),
                })
                expectObservable(service.observeStatus('', SCOPE)).toBe('abcd', {
                    a: null,
                    b: STATUS_1,
                    c: STATUS_2,
                    d: null,
                })
            })
        })

        test('suppresses errors', () => {
            // TODO!(sqs): probably should NOT suppress errors, especially when observing a single status
            scheduler().run(({ expectObservable }) => {
                const service = createStatusService(EMPTY_DIAGNOSTICS, false)
                service.registerStatusProvider('a', {
                    provideStatus: () => throwError(new Error('x')),
                })
                expectObservable(service.observeStatus('', SCOPE)).toBe('a', {
                    a: null,
                })
            })
        })
    })

    describe('registerStatusProvider', () => {
        test('enforces unique registration names', () => {
            const service = createStatusService(EMPTY_DIAGNOSTICS, false)
            service.registerStatusProvider('a', {
                provideStatus: () => of(null),
            })
            expect(() =>
                service.registerStatusProvider('a', {
                    provideStatus: () => of(null),
                })
            ).toThrowError(/already registered/)
        })
    })
})
