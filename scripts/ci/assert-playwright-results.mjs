#!/usr/bin/env node

import { readFileSync } from "node:fs"

const [resultsPath, expectedPassedRaw, expectedSkippedRaw] = process.argv.slice(2)
if (!resultsPath || !expectedPassedRaw || !expectedSkippedRaw) {
  console.error("usage: assert-playwright-results.mjs <json-report> <passed> <skipped>")
  process.exit(2)
}

const expectedPassed = Number(expectedPassedRaw)
const expectedSkipped = Number(expectedSkippedRaw)

let report
try {
  report = JSON.parse(readFileSync(resultsPath, "utf8"))
} catch (error) {
  console.error(`Could not read Playwright JSON report ${resultsPath}: ${error.message}`)
  process.exit(1)
}

const stats = report.stats ?? {}
const passed = stats.expected
const skipped = stats.skipped
const failed = stats.unexpected
const flaky = stats.flaky

if (passed !== expectedPassed || skipped !== expectedSkipped || failed !== 0 || flaky !== 0) {
  console.error(
    `Playwright gate failed: ${passed} passed, ${skipped} skipped, ${failed} failed, ${flaky} flaky; ` +
      `expected ${expectedPassed} passed, ${expectedSkipped} skipped, 0 failed, 0 flaky`,
  )
  process.exit(1)
}

console.log(`Playwright gate passed: ${passed} passed, ${skipped} skipped, 0 failed, 0 flaky`)
