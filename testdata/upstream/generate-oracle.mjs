import { readFile } from 'node:fs/promises'
import { join, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'

const packagePath = process.argv[2]
if (!packagePath)
  throw new Error('Usage: node generate-oracle.mjs <verified-utils-package-directory>')

const packageDirectory = resolve(packagePath)
const metadata = JSON.parse(await readFile(join(packageDirectory, 'package.json'), 'utf8'))
if (metadata.name !== '@iconify/utils' || metadata.version !== '3.1.7')
  throw new Error('The oracle requires the integrity-verified @iconify/utils 3.1.7 archive')

const { getIconData } = await import(pathToFileURL(join(packageDirectory, 'lib/icon-set/get-icon.js')).href)
const { iconToSVG } = await import(pathToFileURL(join(packageDirectory, 'lib/svg/build.js')).href)
const input = JSON.parse(await readFile(new URL('./alias-transform.input.json', import.meta.url), 'utf8'))
if (input.schemaVersion !== 1)
  throw new Error('Unknown input schema version')

const results = input.cases.map(({ id, icon, customisations }) => {
  const resolved = getIconData(input.collection, icon)
  return {
    id,
    resolved,
    svg: resolved === null ? null : iconToSVG(resolved, customisations),
  }
})

process.stdout.write(`${JSON.stringify({ schemaVersion: 1, results }, null, 2)}\n`)
