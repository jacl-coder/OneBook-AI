import { useMemo, useState } from 'react'

import { DocumentReader } from './DocumentReader'
import type { DocumentCitationTarget, DocumentReaderProfile, DocumentReaderSource } from './types'

const demoText = `实习证明

实习人员资料：
兹证明 兰州理工大学 学校，学生 赖新鹏 性别 男 在 我单位内 工程研发 部门 工程研发 岗位进行实习工作，实习时间：2024 年 8 月 1 日开始至 2024 年 12 月 30 日截止。

该同学团队意识强，沟通协作能力优秀，与同事相处融洽，赢得了一致好评，是一位可靠且值得信赖的团队成员。

特此证明！

实习单位（盖章）
2025 年 1 月 3 日`

const demoProfile: DocumentReaderProfile = {
  typeLabel: '实习证明',
  summary:
    '这是一份实习证明，说明赖新鹏在工程研发部门、工程研发岗位完成了 2024 年 8 月 1 日至 2024 年 12 月 30 日的实习。',
  entities: [
    { label: '学生', value: '赖新鹏' },
    { label: '学校', value: '兰州理工大学' },
    { label: '部门', value: '工程研发' },
  ],
  dates: [
    { label: '开始时间', value: '2024-08-01' },
    { label: '结束时间', value: '2024-12-30' },
    { label: '证明日期', value: '2025-01-03' },
  ],
  facts: [
    { label: 'student_name', value: '赖新鹏' },
    { label: 'internship_department', value: '工程研发' },
    { label: 'internship_role', value: '工程研发' },
  ],
}

const demoCitations: DocumentCitationTarget[] = [
  {
    id: 'student-name',
    label: 'page 1',
    lineStart: 4,
    lineEnd: 4,
    snippet: '兹证明 兰州理工大学 学校，学生 赖新鹏 性别 男...',
    sourceReason: '直接包含学生姓名字段。',
    sourceType: 'fact',
    highlightText: '赖新鹏',
  },
  {
    id: 'internship-time',
    label: 'page 1',
    lineStart: 4,
    lineEnd: 4,
    snippet: '实习时间：2024 年 8 月 1 日开始至 2024 年 12 月 30 日截止。',
    sourceReason: '直接包含实习开始和结束时间。',
    sourceType: 'chunk',
    highlightText: '2024 年 8 月 1 日',
  },
  {
    id: 'department',
    label: 'page 1',
    lineStart: 4,
    lineEnd: 4,
    snippet: '在 我单位内 工程研发 部门 工程研发 岗位进行实习工作。',
    sourceReason: '直接包含部门和岗位。',
    sourceType: 'fact',
    highlightText: '工程研发',
  },
]

export function DocumentReaderDemo() {
  const [activeCitationId, setActiveCitationId] = useState(demoCitations[0]?.id)
  const source = useMemo<DocumentReaderSource>(
    () => ({
      id: 'demo-internship-proof',
      title: '赖新鹏实习证明.txt',
      format: 'txt',
      text: demoText,
    }),
    [],
  )

  return (
    <DocumentReader
      activeCitationId={activeCitationId}
      citations={demoCitations}
      onCitationSelect={(citation) => setActiveCitationId(citation.id)}
      profile={demoProfile}
      source={source}
    />
  )
}
