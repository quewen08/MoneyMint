/**
 * 后端端点路径常量（批 0 起，批 4 扩至 36 条）
 *
 * 与 `internal/server/router.go` 的 `mux.HandleFunc("/api/...")` 一一对应；
 * 同一路径上的多 HTTP 方法（如 GET/POST /api/accounts）共用一个常量。
 * 真正的请求实现（fetch 封装、X-Ledger-Id 注入、错误映射）在批 1 落地。
 */
export const API_ROUTES = {
  /** GET /api/health */
  health: '/api/health',

  /** POST /api/auth/register */
  register: '/api/auth/register',
  /** POST /api/auth/login */
  login: '/api/auth/login',
  /** POST /api/auth/logout */
  logout: '/api/auth/logout',
  /** GET /api/auth/me */
  me: '/api/auth/me',
  /** POST /api/auth/refresh */
  refresh: '/api/auth/refresh',
  /** POST /api/auth/change-password */
  changePassword: '/api/auth/change-password',
  /** GET /api/auth/sessions */
  sessions: '/api/auth/sessions',
  /** POST /api/auth/sessions/revoke-others */
  sessionsRevokeOthers: '/api/auth/sessions/revoke-others',
  /** DELETE /api/auth/sessions/{sid} */
  sessionBySid: '/api/auth/sessions/{sid}',

  /** GET /api/accounts · POST /api/accounts */
  accounts: '/api/accounts',
  /** GET /api/accounts/{uuid}/opening · POST /api/accounts/{uuid}/opening */
  accountOpening: '/api/accounts/{uuid}/opening',
  /** DELETE /api/accounts/{uuid} */
  accountByUuid: '/api/accounts/{uuid}',
  /** GET /api/identity/check */
  identityCheck: '/api/identity/check',

  /** GET /api/transactions · POST /api/transactions */
  transactions: '/api/transactions',
  /** GET /api/transactions/trash */
  transactionsTrash: '/api/transactions/trash',
  /** POST /api/transactions/{uuid}/restore */
  transactionRestore: '/api/transactions/{uuid}/restore',
  /** DELETE /api/transactions/{uuid} */
  transactionByUuid: '/api/transactions/{uuid}',

  /** GET /api/ledgers · POST /api/ledgers */
  ledgers: '/api/ledgers',
  /** PATCH /api/ledgers/{id} */
  ledgerById: '/api/ledgers/{id}',
  /** GET /api/ledger/members · POST /api/ledger/members */
  ledgerMembers: '/api/ledger/members',
  /** POST /api/ledger/members/{userID} · DELETE /api/ledger/members/{userID} */
  ledgerMember: '/api/ledger/members/{userID}',
  /** POST /api/ledger/members/{userID}/reset-password */
  ledgerMemberResetPassword: '/api/ledger/members/{userID}/reset-password',

  /** GET /api/export */
  export: '/api/export',
  /** GET /api/export/archive */
  exportArchive: '/api/export/archive',

  /** GET /api/sync/pull */
  syncPull: '/api/sync/pull',
  /** POST /api/sync/push */
  syncPush: '/api/sync/push',

  /** GET /api/backup/list */
  backupList: '/api/backup/list',
  /** GET /api/backup/download?name= */
  backupDownload: '/api/backup/download',
  /** POST /api/backup/run */
  backupRun: '/api/backup/run',
  /** POST /api/backup/restore/preview（M2-b，multipart 上传备份文件，只读预检） */
  backupRestorePreview: '/api/backup/restore/preview',
  /** POST /api/backup/restore（M2-b，multipart + confirm 确认词，执行全库还原） */
  backupRestore: '/api/backup/restore',

  /** POST /api/import/beancount/preview（M2-e，JSON {"text"}，只读解析预览） */
  importBeancountPreview: '/api/import/beancount/preview',
  /** POST /api/import/beancount（M2-e，JSON {"text"}，导入为新账本） */
  importBeancount: '/api/import/beancount',

  /** GET /api/db/instance（批 4 迁移 014：库世代，restore 后变化） */
  dbInstance: '/api/db/instance',

  /** GET /api/settings/registration · POST /api/settings/registration */
  settingsRegistration: '/api/settings/registration',
} as const

/** 路由常量名联合类型。 */
export type ApiRouteName = keyof typeof API_ROUTES

/**
 * 无需携带 `X-Ledger-Id` 的路径（health、auth/*、ledgers、backup/*、settings/*）。
 *
 * 说明：
 * - auth/* 是自服务端点，与具体账本无关；
 * - /api/ledgers 是账本清单 / 账本管理，账本 id 在路径或请求体里；
 * - backup/* 是全库级操作，不解析账本上下文；
 * - settings/* 是全局设置。
 * 其余业务端点缺失 `X-Ledger-Id` 时，批 1 的客户端必须直接抛错，不得静默回退默认账本。
 */
export const NO_LEDGER_PATHS: readonly string[] = [
  '/api/health',
  '/api/auth',
  '/api/ledgers',
  '/api/backup',
  '/api/settings',
  // M2：导入按当前用户新建账本（ledger_id 在响应里），不解析既有账本上下文；
  // db/instance 是只读库标识。
  '/api/import',
  '/api/db',
]

/** 判断给定路径是否无需账本上下文（忽略 query 与 hash）。 */
export function isNoLedgerPath(path: string): boolean {
  const cleanPath = path.split('?')[0].split('#')[0]
  return NO_LEDGER_PATHS.some(
    (prefix) => cleanPath === prefix || cleanPath.startsWith(`${prefix}/`),
  )
}
