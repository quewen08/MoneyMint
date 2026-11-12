/**
 * 用例数下限防假绿（RK-10）：读取 Vitest JSON 报告，断言总用例数不低于下限。
 *
 * 背景：CI 门禁若只看退出码，「测试文件加载失败导致只跑到 3 个用例」也是全绿。
 * 每次新增/删除用例后如需调整下限，改 docker-image.yml 里的 FLOOR 参数即可。
 *
 * 用法：node scripts/check-test-floor.mjs <vitest-report.json> <floor>
 */
import { readFileSync } from 'node:fs'

const [file, floorArg] = process.argv.slice(2)
if (file === undefined || floorArg === undefined) {
  console.error('用法：node scripts/check-test-floor.mjs <vitest-report.json> <floor>')
  process.exit(2)
}

let report
try {
  report = JSON.parse(readFileSync(file, 'utf-8'))
} catch (err) {
  console.error(`❌ 无法读取 Vitest 报告 ${file}: ${err.message}`)
  process.exit(1)
}

const total = Number(report.numTotalTests)
const failed = Number(report.numFailedTests)
const floor = Number(floorArg)

if (!Number.isFinite(total)) {
  console.error('❌ 报告缺少 numTotalTests 字段，报告形态可能已变化，请检查')
  process.exit(1)
}
if (failed > 0) {
  console.error(`❌ 存在 ${failed} 个失败用例，先用例全绿再谈下限`)
  process.exit(1)
}
if (total < floor) {
  console.error(`❌ 用例数 ${total} 低于下限 ${floor}——疑似测试文件加载失败造成的假绿，禁止合入`)
  process.exit(1)
}
console.log(`✅ 用例数 ${total} ≥ 下限 ${floor}，且零失败`)
