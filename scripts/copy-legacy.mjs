#!/usr/bin/env node
/**
 * 把 Flutter 旧版构建产物复制到 frontend/dist/legacy。
 *
 * 为什么用复制而不是移动：批 0~5 期间旧版主流程仍可用，
 * `frontend/build/legacy-web` 作为唯一构建源保留，dist 可随时重建。
 */
import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const source = resolve(root, 'frontend/build/legacy-web')
const target = resolve(root, 'frontend/dist/legacy')

if (!existsSync(resolve(source, 'index.html'))) {
  console.error(`[copy-legacy] 未找到旧版产物: ${source}/index.html`)
  console.error('[copy-legacy] 请先执行 flutter build web --release --base-href /legacy/')
  process.exit(1)
}

rmSync(target, { recursive: true, force: true })
mkdirSync(dirname(target), { recursive: true })
cpSync(source, target, { recursive: true })
console.log(`[copy-legacy] 已复制 ${source} -> ${target}`)
