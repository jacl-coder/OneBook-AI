import { useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { getApiErrorMessage } from '@/features/auth/api/auth'
import {
  adminQueryKeys,
  cancelAdminEvalRun,
  createAdminEvalDataset,
  createAdminEvalRun,
  deleteAdminEvalDataset,
  getAdminEvalOverview,
  getAdminEvalPerQuery,
  getAdminEvalRun,
  listAdminEvalDatasets,
  listAdminEvalRuns,
  type CreateAdminEvalDatasetPayload,
  type CreateAdminEvalRunPayload,
  type ListAdminEvalDatasetsParams,
  type ListAdminEvalRunsParams,
} from '@/features/admin/api/admin'

const tw = {
  notice: 'rounded-[12px] border border-[#f4b0b4] bg-[#fff5f6] px-3 py-[10px] text-[13px] text-[#9f1820]',
  shell: 'grid gap-4',
  cards: 'grid gap-3 md:grid-cols-4',
  card: 'rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4',
  label: 'text-[12px] text-[#6f6f6f]',
  value: 'mt-1 text-[24px] leading-[1.2] font-semibold text-[#0d0d0d]',
  panel: 'rounded-[14px] border border-[rgba(0,0,0,0.08)] bg-white p-4',
  title: 'text-[15px] font-medium',
  grid2: 'grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]',
  form: 'mt-3 grid gap-3',
  input:
    'h-10 rounded-[10px] border border-[rgba(0,0,0,0.12)] px-3 text-[14px] outline-none focus:border-[#111111]',
  textarea:
    'min-h-[88px] rounded-[10px] border border-[rgba(0,0,0,0.12)] px-3 py-2 text-[14px] outline-none focus:border-[#111111]',
  button:
    'inline-flex h-10 items-center justify-center rounded-[10px] bg-[#111111] px-4 text-[13px] font-medium text-white disabled:opacity-50',
  buttonGhost:
    'inline-flex h-9 items-center justify-center rounded-[10px] border border-[rgba(0,0,0,0.12)] px-3 text-[13px] hover:bg-[#f5f5f5] disabled:opacity-50',
  list: 'mt-3 grid gap-2',
  row: 'rounded-[12px] border border-[rgba(0,0,0,0.08)] bg-[#fafafa] p-3',
  rowTop: 'flex items-start justify-between gap-3',
  small: 'text-[12px] text-[#666]',
  chips: 'mt-2 flex flex-wrap gap-2',
  chip: 'rounded-[999px] bg-[#ececec] px-2 py-1 text-[11px] text-[#333]',
  tableWrap: 'mt-3 overflow-auto rounded-[12px] border border-[rgba(0,0,0,0.08)]',
  table: 'min-w-full border-collapse text-[13px]',
  th: 'border-b border-[rgba(0,0,0,0.08)] bg-[#fafafa] px-3 py-2 text-left font-medium text-[#555]',
  td: 'border-b border-[rgba(0,0,0,0.06)] px-3 py-2 align-top',
}

function initialDatasetFilters(): ListAdminEvalDatasetsParams {
  return { page: 1, pageSize: 20, sourceType: '', status: '' }
}

function initialRunFilters(): ListAdminEvalRunsParams {
  return { page: 1, pageSize: 20, status: '', mode: '', retrievalMode: '' }
}

function metricValue(input: unknown): string {
  if (typeof input === 'number') {
    return Number.isInteger(input) ? String(input) : input.toFixed(3)
  }
  return String(input ?? '-')
}

export function AdminEvalsPage() {
  const queryClient = useQueryClient()
  const [datasetFilters, setDatasetFilters] = useState<ListAdminEvalDatasetsParams>(initialDatasetFilters)
  const [runFilters] = useState<ListAdminEvalRunsParams>(initialRunFilters)
  const [selectedRunId, setSelectedRunId] = useState('')
  const [datasetForm, setDatasetForm] = useState<CreateAdminEvalDatasetPayload>({
    name: '',
    sourceType: 'upload',
    version: 1,
    description: '',
    bookId: '',
  })
  const [runForm, setRunForm] = useState<CreateAdminEvalRunPayload>({
    datasetId: '',
    mode: 'all',
    retrievalMode: 'hybrid_best',
    gateMode: 'warn',
    params: { topK: 20, contextBudget: 4000 },
  })

  const overviewQuery = useQuery({
    queryKey: adminQueryKeys.evalOverview,
    queryFn: getAdminEvalOverview,
    refetchInterval: 30_000,
  })

  const datasetsQuery = useQuery({
    queryKey: adminQueryKeys.evalDatasets(datasetFilters),
    queryFn: () => listAdminEvalDatasets(datasetFilters),
  })

  const runsQuery = useQuery({
    queryKey: adminQueryKeys.evalRuns(runFilters),
    queryFn: () => listAdminEvalRuns(runFilters),
    refetchInterval: 10_000,
  })

  const selectedRunQuery = useQuery({
    queryKey: adminQueryKeys.evalRun(selectedRunId),
    queryFn: () => getAdminEvalRun(selectedRunId),
    enabled: selectedRunId.length > 0,
    refetchInterval: 10_000,
  })

  const perQuery = useQuery({
    queryKey: adminQueryKeys.evalPerQuery(selectedRunId),
    queryFn: () => getAdminEvalPerQuery(selectedRunId),
    enabled: selectedRunId.length > 0,
  })

  const createDatasetMutation = useMutation({
    mutationFn: createAdminEvalDataset,
    onSuccess: async () => {
      setDatasetForm({ name: '', sourceType: 'upload', version: 1, description: '', bookId: '' })
      await queryClient.invalidateQueries({ queryKey: ['admin', 'evals'] })
    },
  })

  const deleteDatasetMutation = useMutation({
    mutationFn: deleteAdminEvalDataset,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['admin', 'evals'] })
    },
  })

  const createRunMutation = useMutation({
    mutationFn: createAdminEvalRun,
    onSuccess: async (run) => {
      setSelectedRunId(run.id)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'evals'] })
    },
  })

  const cancelRunMutation = useMutation({
    mutationFn: cancelAdminEvalRun,
    onSuccess: async (run) => {
      setSelectedRunId(run.id)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'evals'] })
    },
  })

  const datasets = datasetsQuery.data?.items ?? []
  const runs = runsQuery.data?.items ?? []
  const selectedRun = selectedRunQuery.data

  const selectedRunArtifacts = useMemo(() => selectedRun?.artifacts ?? [], [selectedRun?.artifacts])

  function handleCreateDataset(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    void createDatasetMutation.mutate(datasetForm)
  }

  function handleCreateRun(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    void createRunMutation.mutate(runForm)
  }

  return (
    <div className={tw.shell}>
      {createDatasetMutation.isError ? (
        <div className={tw.notice}>{getApiErrorMessage(createDatasetMutation.error, '创建评测数据集失败。')}</div>
      ) : null}
      {createRunMutation.isError ? (
        <div className={tw.notice}>{getApiErrorMessage(createRunMutation.error, '创建评测任务失败。')}</div>
      ) : null}

      <div className={tw.cards}>
        <div className={tw.card}>
          <p className={tw.label}>数据集</p>
          <p className={tw.value}>{overviewQuery.data?.totalDatasets ?? '-'}</p>
        </div>
        <div className={tw.card}>
          <p className={tw.label}>运行总数</p>
          <p className={tw.value}>{overviewQuery.data?.totalRuns ?? '-'}</p>
        </div>
        <div className={tw.card}>
          <p className={tw.label}>成功率</p>
          <p className={tw.value}>{overviewQuery.data ? `${Math.round(overviewQuery.data.successRate * 100)}%` : '-'}</p>
        </div>
        <div className={tw.card}>
          <p className={tw.label}>最近 Gate 失败</p>
          <p className={tw.value}>{overviewQuery.data?.recentGateFailed ?? '-'}</p>
        </div>
      </div>

      <div className={tw.grid2}>
        <div className="grid gap-4">
          <section className={tw.panel}>
            <h2 className={tw.title}>创建数据集</h2>
            <form className={tw.form} onSubmit={handleCreateDataset}>
              <input
                className={tw.input}
                placeholder="数据集名称"
                value={datasetForm.name}
                onChange={(event) => setDatasetForm((prev) => ({ ...prev, name: event.target.value }))}
              />
              <select
                className={tw.input}
                value={datasetForm.sourceType}
                onChange={(event) =>
                  setDatasetForm((prev) => ({ ...prev, sourceType: event.target.value as CreateAdminEvalDatasetPayload['sourceType'] }))
                }
              >
                <option value="upload">上传评测包</option>
                <option value="book">绑定书籍</option>
              </select>
              {datasetForm.sourceType === 'book' ? (
                <input
                  className={tw.input}
                  placeholder="书籍 ID"
                  value={datasetForm.bookId ?? ''}
                  onChange={(event) => setDatasetForm((prev) => ({ ...prev, bookId: event.target.value }))}
                />
              ) : null}
              <textarea
                className={tw.textarea}
                placeholder="说明"
                value={datasetForm.description ?? ''}
                onChange={(event) => setDatasetForm((prev) => ({ ...prev, description: event.target.value }))}
              />
              <label className={tw.small}>
                chunks.jsonl
                <input type="file" accept=".jsonl,.json" onChange={(event) => setDatasetForm((prev) => ({ ...prev, chunks: event.target.files?.[0] ?? null }))} />
              </label>
              <label className={tw.small}>
                queries.jsonl
                <input type="file" accept=".jsonl,.json" onChange={(event) => setDatasetForm((prev) => ({ ...prev, queries: event.target.files?.[0] ?? null }))} />
              </label>
              <label className={tw.small}>
                qrels.tsv/jsonl
                <input type="file" accept=".tsv,.txt,.jsonl,.json" onChange={(event) => setDatasetForm((prev) => ({ ...prev, qrels: event.target.files?.[0] ?? null }))} />
              </label>
              <label className={tw.small}>
                predictions.jsonl
                <input type="file" accept=".jsonl,.json" onChange={(event) => setDatasetForm((prev) => ({ ...prev, predictions: event.target.files?.[0] ?? null }))} />
              </label>
              <button className={tw.button} type="submit" disabled={createDatasetMutation.isPending}>
                {createDatasetMutation.isPending ? '创建中...' : '创建数据集'}
              </button>
            </form>
          </section>

          <section className={tw.panel}>
            <h2 className={tw.title}>创建运行</h2>
            <form className={tw.form} onSubmit={handleCreateRun}>
              <select
                className={tw.input}
                value={runForm.datasetId}
                onChange={(event) => setRunForm((prev) => ({ ...prev, datasetId: event.target.value }))}
              >
                <option value="">选择数据集</option>
                {datasets.map((dataset) => (
                  <option key={dataset.id} value={dataset.id}>
                    {dataset.name} ({dataset.id})
                  </option>
                ))}
              </select>
              <select
                className={tw.input}
                value={runForm.mode}
                onChange={(event) => setRunForm((prev) => ({ ...prev, mode: event.target.value as CreateAdminEvalRunPayload['mode'] }))}
              >
                <option value="all">all</option>
                <option value="retrieval">retrieval</option>
                <option value="post_retrieval">post_retrieval</option>
                <option value="answer">answer</option>
              </select>
              <select
                className={tw.input}
                value={runForm.retrievalMode}
                onChange={(event) =>
                  setRunForm((prev) => ({ ...prev, retrievalMode: event.target.value as CreateAdminEvalRunPayload['retrievalMode'] }))
                }
              >
                <option value="hybrid_best">hybrid_best</option>
                <option value="hybrid_no_rerank">hybrid_no_rerank</option>
                <option value="dense_only">dense_only</option>
                <option value="lexical_only">lexical_only</option>
              </select>
              <select
                className={tw.input}
                value={runForm.gateMode}
                onChange={(event) =>
                  setRunForm((prev) => ({ ...prev, gateMode: event.target.value as CreateAdminEvalRunPayload['gateMode'] }))
                }
              >
                <option value="warn">warn</option>
                <option value="strict">strict</option>
                <option value="off">off</option>
              </select>
              <input
                className={tw.input}
                placeholder="Top K"
                type="number"
                value={String((runForm.params?.topK as number | undefined) ?? 20)}
                onChange={(event) =>
                  setRunForm((prev) => ({
                    ...prev,
                    params: { ...prev.params, topK: Number(event.target.value) || 20 },
                  }))
                }
              />
              <input
                className={tw.input}
                placeholder="Context Budget"
                type="number"
                value={String((runForm.params?.contextBudget as number | undefined) ?? 4000)}
                onChange={(event) =>
                  setRunForm((prev) => ({
                    ...prev,
                    params: { ...prev.params, contextBudget: Number(event.target.value) || 4000 },
                  }))
                }
              />
              <button className={tw.button} type="submit" disabled={createRunMutation.isPending || !runForm.datasetId}>
                {createRunMutation.isPending ? '创建中...' : '创建任务'}
              </button>
            </form>
          </section>
        </div>

        <div className="grid gap-4">
          <section className={tw.panel}>
            <div className="flex items-center justify-between gap-3">
              <h2 className={tw.title}>数据集</h2>
              <input
                className={tw.input}
                placeholder="搜索名称或 ID"
                value={datasetFilters.query ?? ''}
                onChange={(event) => setDatasetFilters((prev) => ({ ...prev, query: event.target.value, page: 1 }))}
              />
            </div>
            <div className={tw.list}>
              {datasets.map((dataset) => (
                <article key={dataset.id} className={tw.row}>
                  <div className={tw.rowTop}>
                    <div>
                      <div className="font-medium">{dataset.name}</div>
                      <div className={tw.small}>
                        {dataset.id} · {dataset.sourceType} · {dataset.status}
                      </div>
                    </div>
                    <button
                      type="button"
                      className={tw.buttonGhost}
                      disabled={deleteDatasetMutation.isPending}
                      onClick={() => void deleteDatasetMutation.mutate(dataset.id)}
                    >
                      删除/归档
                    </button>
                  </div>
                  <div className={tw.chips}>
                    {Object.keys(dataset.files ?? {}).map((key) => (
                      <span key={key} className={tw.chip}>
                        {key}
                      </span>
                    ))}
                  </div>
                </article>
              ))}
              {!datasets.length ? <div className={tw.small}>暂无数据集。</div> : null}
            </div>
          </section>

          <section className={tw.panel}>
            <h2 className={tw.title}>运行记录</h2>
            <div className={tw.list}>
              {runs.map((run) => (
                <article key={run.id} className={tw.row}>
                  <div className={tw.rowTop}>
                    <button type="button" className="text-left" onClick={() => setSelectedRunId(run.id)}>
                      <div className="font-medium">{run.id}</div>
                      <div className={tw.small}>
                        {run.mode} · {run.retrievalMode} · {run.status} · gate {run.gateStatus}
                      </div>
                    </button>
                    <button
                      type="button"
                      className={tw.buttonGhost}
                      disabled={cancelRunMutation.isPending || run.status === 'succeeded' || run.status === 'failed' || run.status === 'canceled'}
                      onClick={() => void cancelRunMutation.mutate(run.id)}
                    >
                      取消
                    </button>
                  </div>
                  <div className={tw.small}>progress {run.progress}%</div>
                </article>
              ))}
              {!runs.length ? <div className={tw.small}>暂无运行记录。</div> : null}
            </div>
          </section>

          <section className={tw.panel}>
            <h2 className={tw.title}>结果详情</h2>
            {!selectedRunId ? <div className={tw.small}>选择一个运行查看详情。</div> : null}
            {selectedRunQuery.isError ? (
              <div className={tw.notice}>{getApiErrorMessage(selectedRunQuery.error, '运行详情加载失败。')}</div>
            ) : null}
            {selectedRun ? (
              <div className="mt-3 grid gap-3">
                <div className={tw.row}>
                  <div className={tw.rowTop}>
                    <div>
                      <div className="font-medium">{selectedRun.id}</div>
                      <div className={tw.small}>
                        {selectedRun.status} · {selectedRun.mode} · {selectedRun.retrievalMode}
                      </div>
                    </div>
                    <div className={tw.small}>gate {selectedRun.gateStatus}</div>
                  </div>
                  {selectedRun.errorMessage ? <div className="mt-2 text-[13px] text-[#a32020]">{selectedRun.errorMessage}</div> : null}
                  {selectedRun.warnings?.length ? (
                    <div className="mt-2 grid gap-1 text-[12px] text-[#8a4b00]">
                      {selectedRun.warnings.map((warning) => (
                        <div key={warning}>{warning}</div>
                      ))}
                    </div>
                  ) : null}
                </div>

                <div className={tw.tableWrap}>
                  <table className={tw.table}>
                    <thead>
                      <tr>
                        <th className={tw.th}>Stage</th>
                        <th className={tw.th}>Metrics</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(selectedRun.stageSummaries ?? []).map((summary) => (
                        <tr key={summary.stage}>
                          <td className={tw.td}>{summary.stage}</td>
                          <td className={tw.td}>
                            <div className="grid gap-1">
                              {Object.entries(summary.metrics ?? {}).slice(0, 8).map(([key, value]) => (
                                <div key={key} className={tw.small}>
                                  {key}: {metricValue(value)}
                                </div>
                              ))}
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                <div className={tw.row}>
                  <div className="font-medium">Artifacts</div>
                  <div className={tw.chips}>
                    {selectedRunArtifacts.map((artifact) => (
                      <a
                        key={artifact.name}
                        href={`/api/admin/evals/runs/${selectedRun.id}/artifacts/${artifact.name}`}
                        className={tw.buttonGhost}
                      >
                        {artifact.name}
                      </a>
                    ))}
                  </div>
                </div>

                <div className={tw.tableWrap}>
                  <table className={tw.table}>
                    <thead>
                      <tr>
                        {Object.keys(perQuery.data?.items?.[0] ?? { qid: '' }).map((key) => (
                          <th key={key} className={tw.th}>
                            {key}
                          </th>
                        ))}
                      </tr>
                    </thead>
                    <tbody>
                      {(perQuery.data?.items ?? []).slice(0, 25).map((row, index) => (
                        <tr key={`${row.qid ?? 'row'}-${index}`}>
                          {Object.entries(row).map(([key, value]) => (
                            <td key={key} className={tw.td}>
                              {metricValue(value)}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : null}
          </section>
        </div>
      </div>
    </div>
  )
}
