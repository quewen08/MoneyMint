/**
 * IndexedDB 实现（生产）——与 MemorySyncStore 同接口 `SyncStore`。
 *
 * §6.6.1 #2：三个原子操作各自在**单个 readwrite 事务**里跨
 * accounts / txns / pending / meta / uuid_map 五个 store 完成；
 * 事务失败自动回滚，崩溃只能落在事务边界外、重放幂等。
 *
 * Safari 14.1 兼容：只用最基础的 IDB API（open / transaction / objectStore /
 * get / put / delete / getAll + IDBKeyRange），不用 cursor 高级特性。
 */
import type { LocalAccount, LocalTxn, PendingChange, PendingEntry } from './models'
import type { SyncChange } from '@/api/types'
import {
  backoffMs,
  type ApplyPushResultsInput,
  type MemorySyncStore,
  type RecordEntryInput,
  type SyncStore,
} from './localStore'

const DB_NAME = 'fambean-sync'
const DB_VERSION = 1
const S_ACCOUNTS = 'accounts'
const S_TXNS = 'txns'
const S_PENDING = 'pending'
const S_META = 'meta'
const S_UUID_MAP = 'uuid_map'

const ALL_STORES = [S_ACCOUNTS, S_TXNS, S_PENDING, S_META, S_UUID_MAP]

function openDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, DB_VERSION)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(S_ACCOUNTS)) db.createObjectStore(S_ACCOUNTS)
      if (!db.objectStoreNames.contains(S_TXNS)) db.createObjectStore(S_TXNS)
      if (!db.objectStoreNames.contains(S_PENDING)) {
        db.createObjectStore(S_PENDING, { keyPath: 'id', autoIncrement: true })
      }
      if (!db.objectStoreNames.contains(S_META)) db.createObjectStore(S_META)
      if (!db.objectStoreNames.contains(S_UUID_MAP)) db.createObjectStore(S_UUID_MAP)
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error ?? new Error('IndexedDB open failed'))
  })
}

function reqAs<T>(req: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error ?? new Error('IndexedDB request failed'))
  })
}

export class IdbSyncStore implements SyncStore {
  readonly persistent = true
  private constructor(private db: IDBDatabase) {}

  static async open(): Promise<IdbSyncStore> {
    return new IdbSyncStore(await openDB())
  }

  private entityKey(ledgerId: number, uuid: string): string {
    return `${ledgerId}:${uuid}`
  }

  private metaKey(name: string): string {
    return name
  }

  /** 单 readwrite 事务跑完一批操作；任一步失败整体回滚。 */
  private tx<T>(stores: string[], body: (t: IDBTransaction) => Promise<T>): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      const tx = this.db.transaction(stores, 'readwrite')
      let result: T
      tx.oncomplete = () => resolve(result)
      tx.onerror = () => reject(tx.error ?? new Error('IndexedDB tx failed'))
      tx.onabort = () => reject(tx.error ?? new Error('IndexedDB tx aborted'))
      body(tx)
        .then((r) => {
          result = r
        })
        .catch((cause) => {
          try {
            tx.abort()
          } catch {
            /* 已提交/中止时忽略 */
          }
          reject(cause)
        })
    })
  }

  async getAllAccounts(ledgerId: number): Promise<LocalAccount[]> {
    const tx = this.db.transaction(S_ACCOUNTS, 'readonly')
    const rows = await reqAs(tx.objectStore(S_ACCOUNTS).getAll() as IDBRequest<Array<{ k: string; v: LocalAccount }>>)
    const prefix = `${ledgerId}:`
    return rows.filter((r) => r.k.startsWith(prefix)).map((r) => r.v)
  }

  async getAllTxns(ledgerId: number): Promise<LocalTxn[]> {
    const tx = this.db.transaction(S_TXNS, 'readonly')
    const rows = await reqAs(tx.objectStore(S_TXNS).getAll() as IDBRequest<Array<{ k: string; v: LocalTxn }>>)
    const prefix = `${ledgerId}:`
    return rows
      .filter((r) => r.k.startsWith(prefix))
      .map((r) => r.v)
      .sort((a, b) => (a.date < b.date ? 1 : a.date > b.date ? -1 : 0))
  }

  async getAccount(ledgerId: number, uuid: string): Promise<LocalAccount | null> {
    const tx = this.db.transaction(S_ACCOUNTS, 'readonly')
    const row = await reqAs(
      tx.objectStore(S_ACCOUNTS).get(this.entityKey(ledgerId, uuid)) as IDBRequest<
        { k: string; v: LocalAccount } | undefined
      >,
    )
    return row === undefined ? null : row.v
  }

  async getClientId(): Promise<string> {
    const tx = this.db.transaction(S_META, 'readonly')
    const row = await reqAs(
      tx.objectStore(S_META).get(this.metaKey('client_id')) as IDBRequest<{ v: string } | undefined>,
    )
    if (row !== undefined) return row.v
    const id = `web-${Date.now()}`
    await this.tx([S_META], async (t) => {
      t.objectStore(S_META).put({ v: id }, this.metaKey('client_id'))
      await Promise.resolve()
    })
    return id
  }

  async getLastSeq(ledgerId: number): Promise<number> {
    const tx = this.db.transaction(S_META, 'readonly')
    const row = await reqAs(
      tx.objectStore(S_META).get(this.metaKey(`seq.${ledgerId}`)) as IDBRequest<{ v: number } | undefined>,
    )
    return row === undefined ? 0 : row.v
  }

  async getInstanceId(ledgerId: number): Promise<string | null> {
    const tx = this.db.transaction(S_META, 'readonly')
    const row = await reqAs(
      tx.objectStore(S_META).get(this.metaKey(`instance.${ledgerId}`)) as IDBRequest<
        { v: string } | undefined
      >,
    )
    return row === undefined ? null : row.v
  }

  async setInstanceId(ledgerId: number, id: string): Promise<void> {
    await this.tx([S_META], async (t) => {
      t.objectStore(S_META).put({ v: id }, this.metaKey(`instance.${ledgerId}`))
      await Promise.resolve()
    })
  }

  async recordEntry(ledgerId: number, input: RecordEntryInput): Promise<number> {
    return this.tx([S_ACCOUNTS, S_TXNS, S_PENDING], async (t) => {
      if (input.account !== undefined) {
        t.objectStore(S_ACCOUNTS).put(
          { k: this.entityKey(ledgerId, input.account.uuid), v: input.account },
          this.entityKey(ledgerId, input.account.uuid),
        )
      }
      if (input.txn !== undefined) {
        t.objectStore(S_TXNS).put(
          { k: this.entityKey(ledgerId, input.txn.uuid), v: input.txn },
          this.entityKey(ledgerId, input.txn.uuid),
        )
      }
      const entry: Omit<PendingEntry, 'id'> = {
        state: 'pending',
        attempts: 0,
        next_retry_at: 0,
        change: input.pending,
      }
      const id = await reqAs(t.objectStore(S_PENDING).add(entry) as IDBRequest<IDBValidKey>)
      return Number(id)
    })
  }

  async applyPushResults(ledgerId: number, input: ApplyPushResultsInput): Promise<void> {
    await this.tx([...ALL_STORES], async (t) => {
      const mapStore = t.objectStore(S_UUID_MAP)
      const mapRow =
        (await reqAs(mapStore.get(String(ledgerId)) as IDBRequest<Record<string, string> | undefined>)) ?? {}
      const accStore = t.objectStore(S_ACCOUNTS)
      const txnStore = t.objectStore(S_TXNS)

      for (const [localUuid, serverUuid] of Object.entries(input.uuidMap)) {
        // §6.6.1 #3：映射先落库（与删 pending 同事务），崩溃后可回放。
        mapRow[localUuid] = serverUuid
        // 实体重映射。
        const accRow = (await reqAs(
          accStore.get(this.entityKey(ledgerId, localUuid)) as IDBRequest<
            { k: string; v: LocalAccount } | undefined
          >,
        )) ?? null
        if (accRow !== null) {
          accStore.delete(accRow.k)
          accStore.put({ k: this.entityKey(ledgerId, serverUuid), v: { ...accRow.v, uuid: serverUuid } })
        }
        const txnRows = (await reqAs(
          txnStore.getAll() as IDBRequest<Array<{ k: string; v: LocalTxn }>>,
        )) ?? []
        for (const row of txnRows) {
          if (!row.k.startsWith(`${ledgerId}:`)) continue
          if (!row.v.postings.some((p) => p.account_uuid === localUuid)) continue
          txnStore.put(
            {
              k: row.k,
              v: {
                ...row.v,
                postings: row.v.postings.map((p) =>
                  p.account_uuid === localUuid ? { ...p, account_uuid: serverUuid } : p,
                ),
              },
            },
            row.k,
          )
        }
      }
      if (Object.keys(mapRow).length > 0) {
        mapStore.put(mapRow, String(ledgerId))
      }

      const pendingStore = t.objectStore(S_PENDING)
      for (const id of input.removedIds) {
        pendingStore.delete(id)
      }
      for (const u of input.updates) {
        const row = (await reqAs(
          pendingStore.get(u.id) as IDBRequest<{ id: number; state: string; v: PendingChange } | undefined>,
        )) ?? null
        if (row !== null) {
          pendingStore.put({ ...row, v: u.change }, u.id)
        }
      }

      // 水位单调推进（回退不写）。
      const metaStore = t.objectStore(S_META)
      const seqRow = (await reqAs(
        metaStore.get(this.metaKey(`seq.${ledgerId}`)) as IDBRequest<{ v: number } | undefined>,
      )) ?? undefined
      const cur = seqRow === undefined ? 0 : seqRow.v
      if (input.checkpoint > cur) {
        metaStore.put({ v: input.checkpoint }, this.metaKey(`seq.${ledgerId}`))
      }
    })
  }

  async applyPullChanges(ledgerId: number, changes: SyncChange[], checkpoint: number): Promise<void> {
    await this.tx([S_ACCOUNTS, S_TXNS, S_META], async (t) => {
      const accStore = t.objectStore(S_ACCOUNTS)
      const txnStore = t.objectStore(S_TXNS)
      for (const c of changes) {
        const uuid = c.entity['uuid']
        if (typeof uuid !== 'string' || uuid === '') continue
        const rawOp = c.op as string | undefined
        const op: string = rawOp === undefined || rawOp === '' ? 'create' : rawOp
        const key = this.entityKey(ledgerId, uuid)
        if (op === 'create' || op === 'update') {
          if (c.entity_type === 'account') {
            const prev =
              (await reqAs(accStore.get(key) as IDBRequest<{ k: string; v: LocalAccount } | undefined>)) ?? null
            accStore.put(
              { k: key, v: { ...(prev?.v ?? {}), ...(c.entity as unknown as LocalAccount) } },
              key,
            )
          } else if (c.entity_type === 'transaction') {
            txnStore.put({ k: key, v: c.entity as unknown as LocalTxn }, key)
          }
        } else if (op === 'close') {
          const prev =
            (await reqAs(accStore.get(key) as IDBRequest<{ k: string; v: LocalAccount } | undefined>)) ?? null
          if (prev !== null) {
            accStore.put(
              {
                k: key,
                v: { ...prev.v, close_date: (c.entity['close_date'] as string | undefined) ?? '' },
              },
              key,
            )
          }
        } else if (op === 'delete') {
          if (c.entity_type === 'transaction') {
            txnStore.delete(key)
          } else if (c.entity_type === 'account') {
            const prev =
              (await reqAs(accStore.get(key) as IDBRequest<{ k: string; v: LocalAccount } | undefined>)) ?? null
            if (prev !== null) {
              accStore.put(
                {
                  k: key,
                  v: { ...prev.v, close_date: (c.entity['close_date'] as string | undefined) ?? '' },
                },
                key,
              )
            }
          }
        }
      }
      const metaStore = t.objectStore(S_META)
      const seqRow = (await reqAs(
        metaStore.get(this.metaKey(`seq.${ledgerId}`)) as IDBRequest<{ v: number } | undefined>,
      )) ?? undefined
      const cur = seqRow === undefined ? 0 : seqRow.v
      if (checkpoint > cur) {
        metaStore.put({ v: checkpoint }, this.metaKey(`seq.${ledgerId}`))
      }
    })
  }

  async getPending(ledgerId: number, state?: 'pending' | 'dead'): Promise<PendingEntry[]> {
    const tx = this.db.transaction(S_PENDING, 'readonly')
    const rows =
      (await reqAs(
        tx.objectStore(S_PENDING).getAll() as IDBRequest<
          Array<{ id: number; state: 'pending' | 'dead'; attempts: number; next_retry_at: number; last_error_code?: string; last_error?: string; v: PendingChange }>
        >,
      )) ?? []
    const out = rows.map((r) => ({
      id: r.id,
      state: r.state,
      attempts: r.attempts,
      next_retry_at: r.next_retry_at,
      last_error_code: r.last_error_code,
      last_error: r.last_error,
      change: r.v,
    }))
    return out
      .filter((e) => state === undefined || e.state === state)
      .sort((a, b) => a.id - b.id)
  }

  async countPending(ledgerId: number, state?: 'pending' | 'dead'): Promise<number> {
    return (await this.getPending(ledgerId, state)).length
  }

  async markDead(ledgerId: number, ids: number[], code: string, error: string): Promise<void> {
    await this.tx([S_PENDING], async (t) => {
      const store = t.objectStore(S_PENDING)
      for (const id of ids) {
        const row = (await reqAs(store.get(id) as IDBRequest<Record<string, unknown> | undefined>)) ?? null
        if (row !== null && row['state'] === 'pending') {
          store.put({ ...row, state: 'dead', last_error_code: code, last_error: error }, id)
        }
      }
      await Promise.resolve()
    })
  }

  async scheduleRetry(
    ledgerId: number,
    id: number,
    attempts: number,
    nextRetryAt: number,
    code: string,
    error: string,
  ): Promise<void> {
    void backoffMs(attempts) // 引擎负责计算；存储层只落盘
    await this.tx([S_PENDING], async (t) => {
      const store = t.objectStore(S_PENDING)
      const row = (await reqAs(store.get(id) as IDBRequest<Record<string, unknown> | undefined>)) ?? null
      if (row !== null && row['state'] === 'pending') {
        store.put(
          { ...row, attempts, next_retry_at: nextRetryAt, last_error_code: code, last_error: error },
          id,
        )
      }
      await Promise.resolve()
    })
  }

  async reviveDead(ledgerId: number, ids: number[]): Promise<void> {
    await this.tx([S_PENDING], async (t) => {
      const store = t.objectStore(S_PENDING)
      for (const id of ids) {
        const row = (await reqAs(store.get(id) as IDBRequest<Record<string, unknown> | undefined>)) ?? null
        if (row !== null && row['state'] === 'dead') {
          store.put({ ...row, state: 'pending', attempts: 0, next_retry_at: 0 }, id)
        }
      }
      await Promise.resolve()
    })
  }

  async discardPending(ledgerId: number, ids: number[]): Promise<void> {
    await this.tx([S_PENDING], async (t) => {
      const store = t.objectStore(S_PENDING)
      for (const id of ids) {
        store.delete(id)
      }
      await Promise.resolve()
    })
  }

  async quarantineAndReset(ledgerId: number, newInstanceId: string): Promise<void> {
    await this.tx([...ALL_STORES], async (t) => {
      const prefix = `${ledgerId}:`
      const accStore = t.objectStore(S_ACCOUNTS)
      const txnStore = t.objectStore(S_TXNS)
      const accRows = (await reqAs(accStore.getAll() as IDBRequest<Array<{ k: string }>>)) ?? []
      for (const row of accRows) {
        if (row.k.startsWith(prefix)) accStore.delete(row.k)
      }
      const txnRows = (await reqAs(txnStore.getAll() as IDBRequest<Array<{ k: string }>>)) ?? []
      for (const row of txnRows) {
        if (row.k.startsWith(prefix)) txnStore.delete(row.k)
      }
      const pendingStore = t.objectStore(S_PENDING)
      const pendingRows =
        (await reqAs(
          pendingStore.getAll() as IDBRequest<Array<Record<string, unknown>>>,
        )) ?? []
      for (const row of pendingRows) {
        if (row['state'] === 'pending') {
          pendingStore.put(
            {
              ...row,
              state: 'dead',
              last_error_code: 'instance_rotated',
              last_error: '服务端账本已被还原，本条待同步变更已隔离（同步页可导出后丢弃或重试）',
            },
            row['id'] as number,
          )
        }
      }
      const metaStore = t.objectStore(S_META)
      metaStore.put({ v: 0 }, this.metaKey(`seq.${ledgerId}`))
      metaStore.put({ v: newInstanceId }, this.metaKey(`instance.${ledgerId}`))
      // uuid_map 是「本地→服务端」映射，库被替换后全部失效。
      t.objectStore(S_UUID_MAP).delete(String(ledgerId))
      await Promise.resolve()
    })
  }

  async getUuidMap(ledgerId: number): Promise<Record<string, string>> {
    const tx = this.db.transaction(S_UUID_MAP, 'readonly')
    const row =
      (await reqAs(
        tx.objectStore(S_UUID_MAP).get(String(ledgerId)) as IDBRequest<Record<string, string> | undefined>,
      )) ?? {}
    return row
  }

  async tryAcquireLease(holder: string, now: number, ttlMs: number): Promise<boolean> {
    const result = await this.tx([S_META], async (t) => {
      const store = t.objectStore(S_META)
      const row = (await reqAs(
        store.get(this.metaKey('lease')) as IDBRequest<{ v: { holder: string; expiresAt: number } } | undefined>,
      )) ?? undefined
      if (row !== undefined && row.v.holder !== holder && row.v.expiresAt > now) {
        return false
      }
      store.put({ v: { holder, expiresAt: now + ttlMs } }, this.metaKey('lease'))
      return true
    })
    return result
  }

  async releaseLease(holder: string): Promise<void> {
    await this.tx([S_META], async (t) => {
      const store = t.objectStore(S_META)
      const row = (await reqAs(
        store.get(this.metaKey('lease')) as IDBRequest<{ v: { holder: string; expiresAt: number } } | undefined>,
      )) ?? undefined
      if (row !== undefined && row.v.holder === holder) {
        store.delete(this.metaKey('lease'))
      }
      await Promise.resolve()
    })
  }
}

// MemorySyncStore 仅作类型关联引用（接口一致性由测试保证）。
export type { MemorySyncStore }
