export function exportCSV(filename: string, ...rows: (string | number)[][]) {
  const headers = rows[0] as string[]
  const dataRows = rows.slice(1) as (string | number)[][]
  const lines = [headers.join(','), ...dataRows.map(r => r.map(v => `"${v}"`).join(','))]
  const blob = new Blob([lines.join('\n')], { type: 'text/csv' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = filename
  a.click()
  URL.revokeObjectURL(a.href)
}
